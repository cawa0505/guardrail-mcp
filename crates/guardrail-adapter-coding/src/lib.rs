pub mod inspect;
pub mod patch;
pub mod verifier;

pub use inspect::{InspectEngine, InspectResult};
pub use patch::{PatchEngine, PatchError};
pub use verifier::CompilerVerifier;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_validate_patch_parity() {
        // Empty search
        assert_eq!(
            PatchEngine::validate_patch("", "replace"),
            Err(PatchError::EmptySearch)
        );

        // Too short (< 10 chars)
        assert_eq!(
            PatchEngine::validate_patch("a", "b"),
            Err(PatchError::TooShort { chars: 2 })
        );

        // Too few lines (< 3 lines)
        assert_eq!(
            PatchEngine::validate_patch("line 1 long text", "line 2 long text"),
            Err(PatchError::TooFewLines { lines: 2 })
        );

        // Valid patch (>= 3 lines and >= 10 chars)
        assert!(PatchEngine::validate_patch(
            "fn old_foo() {\n    // do something\n}",
            "fn new_foo() {\n    // do better\n}"
        )
        .is_ok());
    }

    #[test]
    fn test_apply_patch_content_parity() {
        let orig = "fn main() {\n    println!(\"hello\");\n}\n";
        let search = "println!(\"hello\");";
        let replace = "println!(\"world\");";

        let patched = PatchEngine::apply_patch_content(orig, search, replace).expect("apply patch");
        assert_eq!(patched, "fn main() {\n    println!(\"world\");\n}\n");

        // Search not found
        let err = PatchEngine::apply_patch_content(orig, "missing target", replace);
        assert!(matches!(err, Err(PatchError::SearchNotFound { .. })));
    }

    #[test]
    fn test_inspect_reduction_parity() {
        let code = r#"
// single line comment
/* multi
   line
   comment */
fn test() {
    let x = 1;


    let y = 2;
}
"#;
        let res = InspectEngine::inspect_content("test.rs", code, "skeleton");
        assert_eq!(res.language, "rust");
        assert!(!res.content.contains("single line comment"));
        assert!(!res.content.contains("multi\n   line"));
        assert!(res.content.contains("let x = 1;"));
        assert!(res.content.contains("let y = 2;"));
    }
}
