pub mod fsm;
pub mod model;
pub mod token;

pub use fsm::{FsmError, Phase, StateMachine};
pub use model::{
    Action, Actor, Checkpoint, CompResult, Decision, DecisionKind, EvidenceEngine,
    EvidenceError, EvidenceRecord, GuardError, HardGuard, SoftGuard, SoftVerifier,
    StagingBuf, StateData, StateStore, StorageError, VerificationResult,
};
pub use token::{CommitToken, TokenBindings, TokenError};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_phase_allowed_legacy_parity() {
        // Exact parity with Go TestPhaseAllowed_Allowed
        let allowed = [
            (Phase::Init, "get_status"),
            (Phase::Init, "checkpoint"),
            (Phase::Planning, "inspect_context"),
            (Phase::Planning, "checkpoint"),
            (Phase::Planning, "get_status"),
            (Phase::Executing, "inspect_context"),
            (Phase::Executing, "apply_patch"),
            (Phase::Executing, "checkpoint"),
            (Phase::Executing, "get_status"),
            (Phase::Verifying, "inspect_context"),
            (Phase::Verifying, "get_status"),
            (Phase::Paused, "checkpoint"),
            (Phase::Paused, "get_status"),
            (Phase::Completed, "get_status"),
        ];

        for (phase, action) in allowed {
            assert!(
                phase.is_action_allowed_legacy(action),
                "Phase {:?} should allow legacy action {}",
                phase,
                action
            );
        }

        // Exact parity with Go TestPhaseAllowed_Denied
        let denied = [
            (Phase::Init, "apply_patch"),
            (Phase::Init, "inspect_context"),
            (Phase::Planning, "apply_patch"),
            (Phase::Verifying, "apply_patch"),
            (Phase::Verifying, "checkpoint"),
            (Phase::Paused, "apply_patch"),
            (Phase::Paused, "inspect_context"),
            (Phase::Completed, "apply_patch"),
            (Phase::Completed, "checkpoint"),
            (Phase::Completed, "inspect_context"),
        ];

        for (phase, action) in denied {
            assert!(
                !phase.is_action_allowed_legacy(action),
                "Phase {:?} should deny legacy action {}",
                phase,
                action
            );
        }
    }

    #[test]
    fn test_fsm_transitions() {
        let mut fsm = StateMachine::new(Phase::Init);
        assert_eq!(fsm.current_phase(), Phase::Init);

        assert!(fsm.transition_to(Phase::Planning).is_ok());
        assert_eq!(fsm.current_phase(), Phase::Planning);

        // Cannot skip directly from Planning to Completed
        assert!(fsm.transition_to(Phase::Completed).is_err());

        assert!(fsm.transition_to(Phase::Executing).is_ok());
        assert!(fsm.transition_to(Phase::Verifying).is_ok());
        assert!(fsm.transition_to(Phase::Completed).is_ok());

        // Completed is terminal
        assert!(fsm.transition_to(Phase::Planning).is_err());
    }

    #[test]
    fn test_failed_retry_transition() {
        let mut fsm = StateMachine::new(Phase::Executing);
        assert!(fsm.transition_to(Phase::Failed).is_ok());
        assert_eq!(fsm.current_phase(), Phase::Failed);

        // Can retry back to Planning from Failed
        assert!(fsm.transition_to(Phase::Planning).is_ok());
        assert_eq!(fsm.current_phase(), Phase::Planning);
    }

    #[test]
    fn test_token_lifecycle_parity() {
        let now = 1000;
        let mut tok = CommitToken::new("tok1", "prop_hash", "/ws", "rev1", now, Some(600));

        assert!(tok.is_valid(now + 100));
        assert!(tok.validate(Some("prop_hash"), Some("/ws"), Some("rev1"), now + 100).is_ok());

        // Mismatches
        assert!(tok.validate(Some("wrong_hash"), None, None, now + 100).is_err());
        assert!(tok.validate(None, Some("/wrong_ws"), None, now + 100).is_err());
        assert!(tok.validate(None, None, Some("wrong_rev"), now + 100).is_err());

        // Expiry
        assert!(!tok.is_valid(now + 601));
        assert!(tok.validate(None, None, None, now + 601).is_err());

        // Consumption (one-time use)
        assert!(tok.consume(now + 200).is_ok());
        assert!(tok.used);
        assert!(!tok.is_valid(now + 201));
        assert_eq!(tok.consume(now + 201), Err(TokenError::AlreadyUsed));
    }

    #[test]
    fn test_evidence_hash_chain() {
        let dir = std::env::temp_dir().join(format!("guardrail_test_evidence_{}", std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).unwrap().as_nanos()));
        let file_path = dir.join("evidence.jsonl");
        let engine = EvidenceEngine::new(&file_path);

        let actor = Actor::new("agent-1", "coder", "s1");
        let action1 = Action::new("a1", actor.clone(), "coding.read", "src/main.rs", 100);
        let decision1 = Decision::allow("read permitted", 100);

        let rec1 = EvidenceRecord::new("evt-1", actor.clone(), action1, decision1, 100);
        let (saved1, h1) = engine.append(rec1).expect("append 1");
        assert_eq!(saved1.parent_event, None);

        let action2 = Action::new("a2", actor.clone(), "coding.patch", "src/main.rs", 101);
        let decision2 = Decision::require_approval("sensitive edit", 101);
        let rec2 = EvidenceRecord::new("evt-2", actor, action2, decision2, 101);
        let (saved2, _h2) = engine.append(rec2).expect("append 2");
        assert_eq!(saved2.parent_event, Some(h1));

        let verified_count = engine.verify_chain().expect("verify chain");
        assert_eq!(verified_count, 2);

        // Clean up
        let _ = std::fs::remove_dir_all(dir);
    }

    #[test]
    fn test_hard_guard_decision() {
        let fsm = StateMachine::new(Phase::Planning);
        let actor = Actor::new("agent-1", "coder", "s1");

        // Allowed in Planning
        let action_ok = Action::new("a1", actor.clone(), "inspect_context", "src/lib.rs", 100);
        let d1 = HardGuard::evaluate(&fsm, &action_ok, 100);
        assert_eq!(d1.kind, DecisionKind::Allow);

        // Denied in Planning
        let action_deny = Action::new("a2", actor.clone(), "apply_patch", "src/lib.rs", 100);
        let d2 = HardGuard::evaluate(&fsm, &action_deny, 100);
        assert_eq!(d2.kind, DecisionKind::Deny);

        // Blacklisted file protection
        let fsm_exec = StateMachine::new(Phase::Executing);
        let patch_payload = serde_json::json!({ "path": "/etc/shadow" });
        let action_blacklisted = Action::new("a3", actor, "apply_patch", "/etc/shadow", 100)
            .with_payload(patch_payload);
        let d3 = HardGuard::evaluate(&fsm_exec, &action_blacklisted, 100);
        assert_eq!(d3.kind, DecisionKind::Deny);
    }

    #[test]
    fn test_hard_guard_require_approval_on_sensitive_targets() {
        let fsm = StateMachine::new(Phase::Executing);
        let actor = Actor::new("agent-1", "coder", "s1");

        // Sensitive target -> REQUIRE_APPROVAL, not DENY.
        let action = Action::new("a1", actor.clone(), "apply_patch", "config/server.key", 100);
        let d = HardGuard::evaluate(&fsm, &action, 100);
        assert_eq!(d.kind, DecisionKind::RequireApproval);

        // Normal target in EXECUTING -> ALLOW.
        let ok = Action::new("a2", actor, "apply_patch", "src/main.rs", 100);
        assert_eq!(HardGuard::evaluate(&fsm, &ok, 100).kind, DecisionKind::Allow);
    }
}
