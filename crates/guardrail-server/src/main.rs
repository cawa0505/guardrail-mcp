//! GuardrailMcp server — coding-agent guard over the pure governance core.
//!
//! rmcp stdio transport; tools are 100% parameter-compatible with the Go
//! server (get_status / checkpoint / inspect_context / apply_patch /
//! commit_token). State lives at `.guardrail/state.json` with
//! `.opencode/state.json` fallback (legacy parity).

use std::path::PathBuf;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

mod graphify;
mod softguard;
mod themis;

use guardrail_adapter_coding::inspect::InspectEngine;
use guardrail_adapter_coding::patch::PatchEngine;
use guardrail_adapter_coding::verifier::CompilerVerifier;
use guardrail_core::fsm::{Phase, StateMachine};
use guardrail_core::model::{
    Action, Actor, Checkpoint, CompResult, Decision, DecisionKind, EvidenceEngine, EvidenceRecord,
    HardGuard, StateData, StateStore,
};
use guardrail_core::token::{hash_proposal_content, CommitToken};
use graphify::{GraphifyClient, GraphifyConfig};
use rmcp::model::{
    CallToolRequestParams, CallToolResponse, CallToolResult, ContentBlock, JsonObject,
    ListToolsResult, PaginatedRequestParams, ServerCapabilities, ServerConfig, Tool,
};
use rmcp::service::{RequestContext, RoleServer};
use rmcp::{ErrorData as McpError, ServerHandler, ServiceExt};
use serde_json::{json, Value};
use softguard::{SoftGuardConfig, VerifierInput};
use themis::{ThemisClient, ThemisConfig};
use tokio::sync::Mutex;

fn now_secs() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0)
}

fn obj_schema(properties: Value, required: &[&str]) -> Arc<JsonObject> {
    let mut map = serde_json::Map::new();
    map.insert("type".into(), json!("object"));
    map.insert("properties".into(), properties);
    if !required.is_empty() {
        map.insert("required".into(), json!(required));
    }
    Arc::new(map)
}

struct GuardrailServer {
    root: PathBuf,
    store: StateStore,
    evidence: EvidenceEngine,
    softguard: SoftGuardConfig,
    graphify: GraphifyClient,
    themis: Arc<ThemisClient>,
    state: Mutex<StateData>,
}

impl GuardrailServer {
    fn new() -> Self {
        let root = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
        let store = StateStore::new(&root);
        let state = store.load().unwrap_or_default();
        let state_dir = root.join(".guardrail");
        let evidence = EvidenceEngine::new(state_dir.join("evidence.jsonl"));
        let softguard = SoftGuardConfig::load(&state_dir).unwrap_or_default();
        let graphify = GraphifyClient::new(&GraphifyConfig::load(&state_dir).unwrap_or_default());
        let themis_cfg = ThemisConfig::load(&state_dir).unwrap_or_default();
        let session_id = format!("sess-{}", now_secs());
        let themis = ThemisClient::new(themis_cfg, session_id);
        if themis.is_enabled() {
            let t = themis.clone();
            tokio::spawn(async move {
                let _ = t.fetch_policy().await;
            });
        }
        Self {
            root,
            store,
            evidence,
            softguard,
            graphify,
            themis,
            state: Mutex::new(state),
        }
    }

    fn persist(&self, state: &StateData) -> Result<(), String> {
        self.store
            .save(state)
            .map_err(|e| format!("state save failed: {e}"))
    }

    /// Append one governance decision to the append-only evidence log. The
    /// engine auto-chains `parent_event` to the previous record hash.
    fn record_evidence(&self, actor: &Actor, action: &Action, decision: &Decision) -> Option<String> {
        let ts = now_secs();
        let event_id = format!("{}-{}", action.action_id, ts);
        let record = EvidenceRecord::new(
            event_id.clone(),
            actor.clone(),
            action.clone(),
            decision.clone(),
            ts,
        );
        if self.themis.is_enabled() {
            let dec_str = match decision.kind {
                DecisionKind::Allow => "ALLOW",
                DecisionKind::Deny => "DENY",
                DecisionKind::RequireVerification => "REQUIRE_VERIFICATION",
                DecisionKind::RequireApproval => "REQUIRE_APPROVAL",
            };
            let t_rec = themis::EvidenceRecord {
                event_id,
                parent_event_hash: String::new(),
                event_hash: String::new(),
                timestamp: format!("{ts}"),
                actor: themis::Actor {
                    actor_type: themis::ActorType::Agent,
                    id: actor.agent_id.clone(),
                },
                action: action.action_type.clone(),
                resource: action.target.clone(),
                decision: dec_str.to_string(),
                policy_version: "1".to_string(),
                policy_hash: "local".to_string(),
                verifier: decision.verifier_id.clone(),
                verification_result: Some(decision.reason.clone()),
                input: action.payload.clone(),
                output: Value::Null,
                metadata: json!({}),
            };
            let client = self.themis.clone();
            tokio::spawn(async move {
                client.report_evidence(t_rec).await;
            });
        }
        match self.evidence.append(record) {
            Ok((_, hash)) => Some(hash),
            Err(e) => {
                eprintln!("[guardrail] evidence append failed: {e}");
                None
            }
        }
    }

    fn actor() -> Actor {
        Actor::new("guardrail-server", "coding-agent", "local")
    }

    /// Legacy error envelope — byte-compatible with the Go server's denial shape.
    fn deny(kind: &str, reason: &str, decision: &str) -> CallToolResult {
        CallToolResult::error(vec![ContentBlock::text(
            json!({
                "error": kind,
                "reason": reason,
                "decision": decision,
            })
            .to_string(),
        )])
    }
}

fn tool_defs() -> Vec<Tool> {
    vec![
        Tool::new(
            "get_status",
            "Show current guardrail workflow state, phase, staging buffer and commit token status.",
            obj_schema(json!({}), &[]),
        ),
        Tool::new(
            "checkpoint",
            "Create a progress checkpoint; optionally transition to the next phase.",
            obj_schema(
                json!({
                    "summary": { "type": "string", "description": "Short description of what this phase accomplished." },
                    "next_phase": { "type": "string", "enum": ["PLANNING", "EXECUTING", "VERIFYING", "COMPLETED"], "description": "Optional phase to transition into." }
                }),
                &["summary"],
            ),
        ),
        Tool::new(
            "inspect_context",
            "Safely read file structure. Auto extracts function/struct skeletons with line numbers.",
            obj_schema(
                json!({
                    "path": { "type": "string", "description": "Relative path of the file to inspect." },
                    "mode": { "type": "string", "enum": ["skeleton", "range", "full_cleaned"], "description": "Extraction mode (default skeleton)." },
                    "line_range": { "type": "array", "items": { "type": "integer" }, "minItems": 2, "maxItems": 2, "description": "Start/end line for range mode." }
                }),
                &["path"],
            ),
        ),
        Tool::new(
            "apply_patch",
            "Apply a code patch. Automatically validated by the compiler before writing.",
            obj_schema(
                json!({
                    "path": { "type": "string", "description": "Target file path." },
                    "search_block": { "type": "string", "description": "Exact source snippet to replace." },
                    "replace_block": { "type": "string", "description": "New source snippet." },
                    "auto_commit_checkpoint": { "type": "boolean", "description": "Create a checkpoint after applying." }
                }),
                &["path", "search_block", "replace_block"],
            ),
        ),
        Tool::new(
            "commit_token",
            "Manage commit-token lifecycle: create, validate, consume, revoke, status.",
            obj_schema(
                json!({
                    "action": { "type": "string", "enum": ["create", "validate", "consume", "revoke", "status"], "description": "Lifecycle operation." },
                    "proposal_hash": { "type": "string", "description": "SHA-256 of the proposal (create/validate/consume)." },
                    "token_id": { "type": "string", "description": "Token id (validate/consume/revoke)." },
                    "ttl_minutes": { "type": "integer", "description": "TTL in minutes (create, default 30)." },
                    "workspace_path": { "type": "string", "description": "Workspace binding (create)." }
                }),
                &["action"],
            ),
        ),
    ]
}

impl ServerHandler for GuardrailServer {
    fn get_info(&self) -> ServerConfig {
        ServerConfig::new(ServerCapabilities::builder().enable_tools().build())
            .with_server_info(rmcp::model::Implementation::new("guardrail-mcp", "2.0.0"))
            .with_instructions(
                "Coding-agent guard. State machine INIT -> PLANNING -> EXECUTING -> VERIFYING -> COMPLETED. \
                 apply_patch requires the EXECUTING phase and a valid commit token flow.",
            )
    }

    async fn list_tools(
        &self,
        _request: Option<PaginatedRequestParams>,
        _context: RequestContext<RoleServer>,
    ) -> Result<ListToolsResult, McpError> {
        Ok(ListToolsResult::with_all_items(tool_defs()))
    }

    async fn call_tool(
        &self,
        request: CallToolRequestParams,
        context: RequestContext<RoleServer>,
    ) -> Result<CallToolResponse, McpError> {
        let args = request.arguments.unwrap_or_default();
        let result: CallToolResult = match request.name.as_ref() {
            "get_status" => self.handle_get_status().await?,
            "checkpoint" => self.handle_checkpoint(&args).await?,
            "inspect_context" => self.handle_inspect(&args).await?,
            "apply_patch" => self.handle_apply_patch(&args, &context).await?,
            "commit_token" => self.handle_commit_token(&args).await?,
            other => {
                return Err(McpError::invalid_params(
                    format!("unknown tool: {other}"),
                    None,
                ))
            }
        };
        Ok(result.into())
    }
}

impl GuardrailServer {
    async fn handle_get_status(&self) -> Result<CallToolResult, McpError> {
        let state = self.state.lock().await;
        let token_status = state
            .commit_token
            .as_ref()
            .map(|t| {
                let now = now_secs();
                json!({
                    "id": t.id,
                    "valid": t.is_valid(now),
                    "used": t.used,
                    "revoked": t.revoked,
                    "expires_in_secs": (t.expires_at - now).max(0),
                    "bindings": t.bindings,
                })
            })
            .unwrap_or(Value::Null);
        let payload = json!({
            "version": state.version,
            "phase": state.phase,
            "active_goal": state.active_goal,
            "allowed_actions": state.allowed_actions,
            "staging_buffer": state.staging_buffer,
            "failed_attempts": state.failed_attempts,
            "ast_synced": state.ast_synced,
            "checkpoints": state.checkpoints,
            "commit_token": token_status,
        });
        Ok(CallToolResult::success(vec![ContentBlock::text(
            serde_json::to_string_pretty(&payload).unwrap_or_default(),
        )]))
    }

    async fn handle_checkpoint(
        &self,
        args: &serde_json::Map<String, Value>,
    ) -> Result<CallToolResult, McpError> {
        let summary = args
            .get("summary")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        if summary.is_empty() {
            return Ok(Self::deny(
                "invalid_params",
                "summary is required",
                "checkpoint requires a summary",
            ));
        }
        let next_phase = args.get("next_phase").and_then(Value::as_str).map(String::from);

        let mut state = self.state.lock().await;
        // Gate: checkpoints are a planning/executing/verifying activity.
        let ts = now_secs();
        let actor = Self::actor();
        let action = Action::new("checkpoint", actor.clone(), "checkpoint", "", ts);
        let decision = HardGuard::evaluate(&StateMachine::new(state.phase.clone()), &action, ts);
        if decision.kind == DecisionKind::Deny {
            drop(state);
            self.record_evidence(&actor, &action, &decision);
            return Ok(Self::deny("phase_gate", &decision.reason, "DENY"));
        }

        if let Some(np) = &next_phase {
            let target = Phase::parse(np)
                .ok_or_else(|| McpError::invalid_params(format!("invalid next_phase: {np}"), None))?;
            let mut fsm = StateMachine::new(state.phase.clone());
            if let Err(e) = fsm.transition_to(target.clone()) {
                drop(state);
                let d = Decision::deny(e.to_string(), ts);
                self.record_evidence(&actor, &action, &d);
                return Ok(Self::deny("transition", &e.to_string(), "DENY"));
            }
            if target == Phase::Completed && self.themis.is_enabled() {
                let t = self.themis.clone();
                tokio::spawn(async move {
                    if let Err(e) = t.close_session(None).await {
                        eprintln!("[guardrail] themis close_session anchor error: {e}");
                    }
                });
            }
            state.phase = target;
        }

        let id = format!("checkpoint-{}", state.checkpoints.len() + 1);
        let ts_rfc3339 = chrono_secs();
        let modified_files: Vec<String> =
            state.staging_buffer.target_file.clone().into_iter().collect();
        state.checkpoints.push(Checkpoint {
            id: id.clone(),
            timestamp: ts_rfc3339.clone(),
            summary: summary.clone(),
            modified_files,
        });

        if let Err(e) = self.persist(&state) {
            return Err(McpError::internal_error(e, None));
        }
        drop(state);
        self.record_evidence(&actor, &action, &Decision::allow(summary.clone(), ts));
        Ok(CallToolResult::success(vec![ContentBlock::text(
            json!({ "checkpoint": id, "timestamp": ts_rfc3339, "summary": summary }).to_string(),
        )]))
    }

    async fn handle_inspect(
        &self,
        args: &serde_json::Map<String, Value>,
    ) -> Result<CallToolResult, McpError> {
        let path = args
            .get("path")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        if path.is_empty() {
            return Ok(Self::deny(
                "invalid_params",
                "path is required",
                "inspect_context requires a path",
            ));
        }
        let mode = args
            .get("mode")
            .and_then(Value::as_str)
            .unwrap_or("skeleton")
            .to_string();
        let line_range: Option<(usize, usize)> = args.get("line_range").and_then(|v| {
            v.as_array().and_then(|a| {
                if a.len() == 2 {
                    Some((a[0].as_u64()? as usize, a[1].as_u64()? as usize))
                } else {
                    None
                }
            })
        });

        let ts = now_secs();
        let actor = Self::actor();
        let action = Action::new("inspect_context", actor.clone(), "inspect_context", path.clone(), ts);

        let phase = self.state.lock().await.phase.clone();
        let decision = HardGuard::evaluate(&StateMachine::new(phase), &action, ts);
        if decision.kind == DecisionKind::Deny {
            self.record_evidence(&actor, &action, &decision);
            return Ok(Self::deny("phase_gate", &decision.reason, "DENY"));
        }

        let abs = self.root.join(&path);
        let code = match std::fs::read_to_string(&abs) {
            Ok(c) => c,
            Err(e) => {
                return Ok(Self::deny("io_error", &format!("cannot read {path}: {e}"), "ERROR"));
            }
        };

        // Core supports "skeleton"/"full_cleaned" reduction; every other mode
        // (incl. "range") is returned raw. Range slicing happens here.
        let (text, core_mode) = if mode == "range" {
            match line_range {
                Some((s, e)) => {
                    let sliced: Vec<&str> = code.lines().skip(s.saturating_sub(1)).take(e.saturating_sub(s) + 1).collect();
                    (sliced.join("\n"), "range")
                }
                None => {
                    return Ok(Self::deny(
                        "invalid_params",
                        "range mode requires line_range: [start, end]",
                        "ERROR",
                    ));
                }
            }
        } else {
            (code, mode.as_str())
        };

        let result = InspectEngine::inspect_content(&path, &text, core_mode);
        self.record_evidence(&actor, &action, &Decision::allow(&mode, ts));
        Ok(CallToolResult::success(vec![ContentBlock::text(
            serde_json::to_string_pretty(&result).unwrap_or_default(),
        )]))
    }

    async fn handle_apply_patch(
        &self,
        args: &serde_json::Map<String, Value>,
        context: &RequestContext<RoleServer>,
    ) -> Result<CallToolResult, McpError> {
        let path = args
            .get("path")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        let search = args
            .get("search_block")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        let replace = args
            .get("replace_block")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        let auto_checkpoint = args
            .get("auto_commit_checkpoint")
            .and_then(Value::as_bool)
            .unwrap_or(false);

        if path.is_empty() {
            return Ok(Self::deny(
                "invalid_params",
                "path is required",
                "apply_patch requires a path",
            ));
        }

        let ts = now_secs();
        let actor = Self::actor();
        let action = Action::new("apply_patch", actor.clone(), "apply_patch", path.clone(), ts)
            .with_payload(json!({ "search_block": search, "replace_block": replace }));

        // Hard guard: phase gate (EXECUTING only) + blacklist.
        let phase = self.state.lock().await.phase.clone();
        let decision = HardGuard::evaluate(&StateMachine::new(phase), &action, ts);
        if decision.kind == DecisionKind::Deny {
            self.record_evidence(&actor, &action, &decision);
            return Ok(Self::deny("phase_gate", &decision.reason, "DENY"));
        }
        if decision.kind == DecisionKind::RequireApproval {
            // Route the approval request to Themis (if enabled) or MCP elicitation.
            self.record_evidence(&actor, &action, &decision);
            if let Some(denied) = self.handle_require_approval(context, &actor, &action, &decision).await? {
                return Ok(denied);
            }
        }

        // Validate patch shape.
        if let Err(e) = PatchEngine::validate_patch(&search, &replace) {
            let d = Decision::deny(e.to_string(), ts);
            self.record_evidence(&actor, &action, &d);
            return Ok(Self::deny("patch_validation", &e.to_string(), "DENY"));
        }

        // Read + apply in memory.
        let abs = self.root.join(&path);
        let original = match std::fs::read_to_string(&abs) {
            Ok(c) => c,
            Err(e) => {
                return Ok(Self::deny("io_error", &format!("cannot read {path}: {e}"), "ERROR"));
            }
        };
        let patched = match PatchEngine::apply_patch_content(&original, &search, &replace) {
            Ok(p) => p,
            Err(e) => {
                let d = Decision::deny(e.to_string(), ts);
                self.record_evidence(&actor, &action, &d);
                return Ok(Self::deny("patch_not_found", &e.to_string(), "DENY"));
            }
        };

        // Soft guard: compiler check on the patched content (written first —
        // matching the Go server; a failed check rolls the write back).
        if let Err(e) = std::fs::write(&abs, &patched) {
            return Ok(Self::deny("io_error", &format!("cannot write {path}: {e}"), "ERROR"));
        }
        let (passed, output) = CompilerVerifier::run_check(&self.root)
            .await
            .unwrap_or((true, "verifier failed to run; passed by default".into()));
        if !passed {
            let _ = std::fs::write(&abs, &original);
            let d = Decision::require_verification("compiler", "compiler check failed", ts);
            self.record_evidence(&actor, &action, &d);
            // Go parity: 3 consecutive compiler failures auto-degrade to PAUSED.
            let mut state = self.state.lock().await;
            state.failed_attempts += 1;
            state.staging_buffer.last_compiler_result = Some(CompResult {
                success: false,
                raw_output: output.clone(),
            });
            if state.failed_attempts >= 3 {
                state.phase = Phase::Paused;
                state.allowed_actions = Phase::Paused.default_actions().iter().map(|s| s.to_string()).collect();
                state.failed_attempts = 0;
                let _ = self.persist(&state);
                drop(state);
                return Ok(Self::deny(
                    "auto_paused",
                    "compiler validation failed 3 times — auto-transitioned to PAUSED phase. Use checkpoint to resume.",
                    &output,
                ));
            }
            let attempts = state.failed_attempts;
            let _ = self.persist(&state);
            drop(state);
            return Ok(Self::deny(
                "compiler_check_failed",
                &format!("compiler validation failed (attempt {attempts}/3), patch rolled back"),
                &output,
            ));
        }

        // Track 2 Soft Guard: external HTTP verifiers (required failures block).
        if !self.softguard.verifiers.is_empty() {
            let phase_str = self.state.lock().await.phase.as_str().to_string();
            let input = VerifierInput {
                file_path: path.clone(),
                content: patched.clone(),
                patch: Some(replace.clone()),
                phase: phase_str,
            };
            let results = softguard::run_all(&self.softguard, &input);
            if let Some(failed) = softguard::first_required_failure(&results) {
                let _ = std::fs::write(&abs, &original);
                let reason = if failed.message.is_empty() {
                    failed.error.clone()
                } else {
                    failed.message.clone()
                };
                let d = Decision::require_verification(&failed.verifier_name, reason.clone(), ts);
                self.record_evidence(&actor, &action, &d);
                return Ok(Self::deny("soft_guard_failed", &reason, "REQUIRE_VERIFICATION"));
            }
        }

        // Update staging buffer + optional checkpoint.
        let mut state = self.state.lock().await;
        state.failed_attempts = 0;
        state.staging_buffer.has_pending_patch = false;
        state.staging_buffer.target_file = Some(path.clone());
        state.staging_buffer.patch_content = Some(replace);
        state.staging_buffer.last_compiler_result = Some(CompResult {
            success: true,
            raw_output: output,
        });
        state.ast_synced = false;

        let mut checkpoint_id = None;
        if auto_checkpoint {
            let id = format!("checkpoint-{}", state.checkpoints.len() + 1);
            state.checkpoints.push(Checkpoint {
                id: id.clone(),
                timestamp: chrono_secs(),
                summary: format!("auto-checkpoint after apply_patch on {path}"),
                modified_files: vec![path.clone()],
            });
            checkpoint_id = Some(id);
        }

        if let Err(e) = self.persist(&state) {
            return Err(McpError::internal_error(e, None));
        }
        drop(state);
        self.record_evidence(&actor, &action, &Decision::allow("patch applied + compiler passed", ts));

        // Fire-and-forget AST resync via Graphify (never blocks the patch).
        let mut graphify_triggered = false;
        if self.graphify.is_configured() {
            graphify_triggered = true;
            let client = GraphifyClient::new(&GraphifyConfig::load(&self.root.join(".guardrail")).unwrap_or_default());
            let target = path.clone();
            tokio::spawn(async move {
                if let Err(e) = client.skeleton_extract(&target).await {
                    eprintln!("[guardrail] graphify extract skipped: {e}");
                }
            });
        }

        Ok(CallToolResult::success(vec![ContentBlock::text(
            json!({
                "status": "applied",
                "path": path,
                "checkpoint": checkpoint_id,
                "compiler": "passed",
                "graphify_extract_triggered": graphify_triggered,
            })
            .to_string(),
        )]))
    }

    /// Handle a `RequireApproval` decision via Themis remote human approval
    /// (CONTRACT §3) when enabled, or via MCP elicitation fallback.
    /// Returns `Ok(None)` when approved (allowing `apply_patch` to proceed),
    /// or `Ok(Some(deny_result))` when rejected/timed out/unavailable.
    async fn handle_require_approval(
        &self,
        context: &RequestContext<RoleServer>,
        actor: &Actor,
        action: &Action,
        decision: &Decision,
    ) -> Result<Option<CallToolResult>, McpError> {
        if self.themis.is_enabled() {
            let req_id = format!("req-{}-{}", action.action_id, now_secs());
            let expires_at = format!("{}", now_secs() + 300);
            match self
                .themis
                .request_and_await_approval(
                    &req_id,
                    &action.action_type,
                    &action.target,
                    "hardguard-sensitive",
                    &expires_at,
                    std::time::Duration::from_secs(60),
                )
                .await
            {
                Ok(sig) => {
                    self.record_evidence(
                        actor,
                        action,
                        &Decision::allow(
                            format!("themis approved (req={}, sig={})", sig.request_id, sig.signature),
                            now_secs(),
                        ),
                    );
                    return Ok(None);
                }
                Err(reason) => {
                    return Ok(Some(Self::deny(
                        "themis_approval_denied",
                        &reason,
                        "REQUIRE_APPROVAL",
                    )));
                }
            }
        }

        #[derive(serde::Deserialize, schemars::JsonSchema)]
        struct Approval {
            approved: bool,
            #[serde(default)]
            reason: String,
        }
        rmcp::elicit_safe!(Approval);

        let modes = context.peer.supported_elicitation_modes();
        if modes.is_empty() {
            return Ok(Some(Self::deny(
                "approval_unavailable",
                "client does not support elicitation; sensitive operation denied",
                "REQUIRE_APPROVAL",
            )));
        }

        let message = format!(
            "GuardrailMcp requires approval: {}\nAction: {} on {}",
            decision.reason, action.action_type, action.target
        );

        match context.peer.elicit::<Approval>(message).await {
            Ok(Some(a)) if a.approved => {
                let label = if a.reason.is_empty() { "human approved".to_string() } else { a.reason };
                self.record_evidence(actor, action, &Decision::allow(&label, now_secs()));
                Ok(None)
            }
            Ok(_) => Ok(Some(Self::deny(
                "approval_denied",
                "human rejected or did not approve the sensitive operation",
                "REQUIRE_APPROVAL",
            ))),
            Err(e) => Ok(Some(Self::deny(
                "approval_failed",
                &format!("elicitation failed: {e}"),
                "REQUIRE_APPROVAL",
            ))),
        }
    }

    async fn handle_commit_token(
        &self,
        args: &serde_json::Map<String, Value>,
    ) -> Result<CallToolResult, McpError> {
        let action = args
            .get("action")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        let now = now_secs();

        let mut state = self.state.lock().await;
        let payload: Value = match action.as_str() {
            "create" => {
                let proposal_hash = args
                    .get("proposal_hash")
                    .and_then(Value::as_str)
                    .unwrap_or_default()
                    .to_string();
                if proposal_hash.is_empty() {
                    return Ok(Self::deny("invalid_params", "proposal_hash is required", "ERROR"));
                }
                let workspace = args
                    .get("workspace_path")
                    .and_then(Value::as_str)
                    .map(|s| s.to_string())
                    .unwrap_or_else(|| self.root.to_string_lossy().into_owned());
                let ttl_minutes = args.get("ttl_minutes").and_then(Value::as_i64).unwrap_or(30);
                let ttl_secs = Some(ttl_minutes * 60);
                let id = format!(
                    "ct-{:x}",
                    hash_proposal_content(&proposal_hash)
                        .bytes()
                        .take(8)
                        .map(|b| b as u64)
                        .sum::<u64>()
                );
                let revision = std::fs::read_to_string(self.root.join(".git").join("HEAD"))
                    .map(|h| h.trim().to_string())
                    .unwrap_or_else(|_| "no-git".to_string());
                let token = CommitToken::new(id, proposal_hash, workspace, revision, now, ttl_secs);
                let out = json!({
                    "created": true,
                    "id": token.id,
                    "expires_at": token.expires_at,
                    "bindings": token.bindings,
                });
                state.commit_token = Some(token);
                out
            }
            "validate" | "consume" | "revoke" => {
                let Some(token) = state.commit_token.as_mut() else {
                    return Ok(Self::deny("no_token", "no commit token exists", "ERROR"));
                };
                let proposal_hash = args.get("proposal_hash").and_then(Value::as_str);
                let workspace = args.get("workspace_path").and_then(Value::as_str);
                match action.as_str() {
                    "validate" => match token.validate(proposal_hash, workspace, None, now) {
                        Ok(()) => json!({ "valid": true, "id": token.id }),
                        Err(e) => json!({ "valid": false, "reason": e.to_string() }),
                    },
                    "consume" => match token.consume(now) {
                        Ok(()) => json!({ "consumed": true, "id": token.id }),
                        Err(e) => json!({ "consumed": false, "reason": e.to_string() }),
                    },
                    _ => {
                        token.revoke();
                        json!({ "revoked": true, "id": token.id })
                    }
                }
            }
            "status" => state
                .commit_token
                .as_ref()
                .map(|t| {
                    json!({
                        "id": t.id,
                        "valid": t.is_valid(now),
                        "used": t.used,
                        "revoked": t.revoked,
                        "expires_in_secs": (t.expires_at - now).max(0),
                        "bindings": t.bindings,
                    })
                })
                .unwrap_or(Value::Null),
            other => {
                return Ok(Self::deny(
                    "invalid_params",
                    &format!("unknown action: {other}"),
                    "ERROR",
                ));
            }
        };

        if let Err(e) = self.persist(&state) {
            return Err(McpError::internal_error(e, None));
        }
        drop(state);
        let actor = Self::actor();
        let action_rec = Action::new("commit_token", actor.clone(), "commit_token", "", now);
        self.record_evidence(&actor, &action_rec, &Decision::allow(action.clone(), now));
        Ok(CallToolResult::success(vec![ContentBlock::text(
            serde_json::to_string_pretty(&payload).unwrap_or_default(),
        )]))
    }
}

fn chrono_secs() -> String {
    // RFC3339-style timestamp without pulling chrono into the server.
    format!("{}", now_secs())
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let server = GuardrailServer::new();
    let running = server.serve(rmcp::transport::io::stdio()).await?;
    running.waiting().await?;
    Ok(())
}
