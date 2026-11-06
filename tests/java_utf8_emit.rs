//! javac must compile the emitter's UTF-8 comments under a C locale.
//!
//! The Java emitter writes section references (§) into comments. Under
//! LANG=C, javac defaults to US-ASCII and rejects them unless cuni
//! passes `-encoding UTF-8`. Pins the box-found regression: without the
//! flag, `cuni check --only java` fails on any example under C locale.

use std::path::PathBuf;
use std::process::Command;

fn cuni_bin() -> PathBuf {
    PathBuf::from(env!("CARGO_BIN_EXE_cuni"))
}

/// Java seat passes on a small example with LANG/LC_ALL forced to C.
#[test]
fn check_java_seat_passes_under_c_locale() {
    let output = Command::new(cuni_bin())
        .arg("check")
        .arg("examples/compressors/zeros.cuni")
        .arg("--only")
        .arg("java")
        .arg("--timeout")
        .arg("120")
        .env("LANG", "C")
        .env("LC_ALL", "C")
        .output()
        .expect("spawn cuni");
    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        output.status.success(),
        "java seat must pass under C locale:\n{}",
        stdout
    );
    assert!(
        stdout.contains("exactness: PASS"),
        "expected PASS verdict line:\n{}",
        stdout
    );
}
