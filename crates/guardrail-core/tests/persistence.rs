//! Persistence durability tests: atomic state saves and evidence-chain integrity.

use std::sync::{Arc, Mutex};

use guardrail_core::model::{
    Action, Actor, Decision, EvidenceEngine, EvidenceError, EvidenceRecord, StateData, StateStore,
};

fn sample_record(n: usize) -> EvidenceRecord {
    let actor = Actor::new("agent", "coder", "s1");
    let action = Action::new(format!("a{n}"), actor.clone(), "inspect_context", "f.rs", n as i64);
    let decision = Decision::allow("ok", n as i64);
    EvidenceRecord::new(format!("evt-{n}"), actor, action, decision, n as i64)
}

#[test]
fn concurrent_state_saves_no_torn_file() {
    let dir = tempfile::tempdir().unwrap();
    let store = Arc::new(StateStore::new(dir.path()));

    let handles: Vec<_> = (0..8)
        .map(|i| {
            let store = Arc::clone(&store);
            std::thread::spawn(move || {
                let mut state = StateData::default();
                state.active_goal = format!("goal-{i}");
                store.save(&state).expect("save");
            })
        })
        .collect();

    for h in handles {
        h.join().expect("thread join");
    }

    // Must not be a torn/partial JSON document.
    let loaded = store.load().expect("load must succeed after concurrent saves");
    assert!(loaded.active_goal.starts_with("goal-"));
}

#[test]
fn concurrent_evidence_appends_serialized() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("evidence.jsonl");
    let engine = Arc::new(Mutex::new(EvidenceEngine::new(&path)));

    let handles: Vec<_> = (0..4)
        .map(|t| {
            let engine = Arc::clone(&engine);
            std::thread::spawn(move || {
                for i in 0..25 {
                    let n = t * 100 + i;
                    engine.lock().unwrap().append(sample_record(n)).expect("append");
                }
            })
        })
        .collect();

    for h in handles {
        h.join().expect("thread join");
    }

    let count = engine.lock().unwrap().verify_chain().expect("chain intact");
    assert_eq!(count, 100);
}

#[test]
fn evidence_chain_corruption_detected() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("evidence.jsonl");
    let engine = EvidenceEngine::new(&path);

    for n in 0..3 {
        engine.append(sample_record(n)).expect("append");
    }

    // Tamper the 2nd line's parent_event to a bogus 64-hex value.
    let content = std::fs::read_to_string(&path).unwrap();
    let mut lines: Vec<String> = content.lines().map(|s| s.to_string()).collect();
    let mut second: serde_json::Value = serde_json::from_str(&lines[1]).unwrap();
    second["parent_event"] =
        serde_json::Value::String("f".repeat(64));
    lines[1] = serde_json::to_string(&second).unwrap();
    std::fs::write(&path, lines.join("\n") + "\n").unwrap();

    let result = engine.verify_chain();
    assert!(
        matches!(result, Err(EvidenceError::ChainCorruption { .. })),
        "expected ChainCorruption, got {result:?}"
    );
}

#[test]
fn state_store_opencode_fallback() {
    let dir = tempfile::tempdir().unwrap();
    let opencode = dir.path().join(".opencode");
    std::fs::create_dir_all(&opencode).unwrap();
    let state = StateData::default();
    std::fs::write(
        opencode.join("state.json"),
        serde_json::to_string_pretty(&state).unwrap(),
    )
    .unwrap();

    let store = StateStore::new(dir.path());
    assert!(
        store.state_file_path().ends_with(".opencode/state.json"),
        "expected .opencode fallback, got {:?}",
        store.state_file_path()
    );
    store.load().expect("load via opencode fallback");
}
