use std::collections::HashSet;
use thiserror::Error;

use super::Phase;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum FsmError {
    #[error("illegal state transition from {from:?} to {to:?}")]
    IllegalTransition { from: Phase, to: Phase },
    #[error("action '{action}' is denied in phase {phase:?}")]
    ActionDenied { phase: Phase, action: String },
    #[error("workflow has reached terminal state {0:?}")]
    TerminalState(Phase),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StateMachine {
    current_phase: Phase,
    allowed_actions: HashSet<String>,
}

impl StateMachine {
    pub fn new(initial_phase: Phase) -> Self {
        Self {
            current_phase: initial_phase,
            allowed_actions: HashSet::new(),
        }
    }

    pub fn current_phase(&self) -> Phase {
        self.current_phase
    }

    pub fn transition_to(&mut self, next: Phase) -> Result<(), FsmError> {
        if self.current_phase.is_terminal() {
            return Err(FsmError::TerminalState(self.current_phase));
        }

        if !self.current_phase.can_transition_to(next) {
            return Err(FsmError::IllegalTransition {
                from: self.current_phase,
                to: next,
            });
        }

        self.current_phase = next;
        Ok(())
    }

    pub fn set_allowed_actions(&mut self, actions: impl IntoIterator<Item = impl Into<String>>) {
        self.allowed_actions = actions.into_iter().map(Into::into).collect();
    }

    pub fn is_action_allowed(&self, action: &str) -> bool {
        // If custom allowed_actions configured, check against it; otherwise check legacy phase defaults
        if !self.allowed_actions.is_empty() {
            self.allowed_actions.contains(action)
        } else {
            self.current_phase.is_action_allowed_legacy(action)
        }
    }

    pub fn check_action(&self, action: &str) -> Result<(), FsmError> {
        if self.is_action_allowed(action) {
            Ok(())
        } else {
            Err(FsmError::ActionDenied {
                phase: self.current_phase,
                action: action.to_string(),
            })
        }
    }
}
