//! One-product registry: CuNi has a trunk and seven arms, and the machine
//! registry (`arms.json`) must agree with the human page (`ARMS.md`).
//!
//! A silent drift here is how a product turns back into a pile of repos.
//! This test does not judge an arm's quality — only that every arm still
//! exists, points at a real entry/doc, and is named on the page.

use serde_json::Value;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

const ARM_IDS: &[&str] = &[
    "studio",
    "bank",
    "audit",
    "onchain",
    "exactness",
    "transparency",
    "tap",
];

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn read(name: &str) -> String {
    fs::read_to_string(root().join(name)).unwrap_or_else(|e| panic!("read {name}: {e}"))
}

fn registry() -> Value {
    serde_json::from_str(&read("arms.json")).expect("arms.json must be valid JSON")
}

fn arms(reg: &Value) -> &Vec<Value> {
    reg["arms"]
        .as_array()
        .expect("arms.json must have an `arms` array")
}

fn field<'a>(arm: &'a Value, key: &str) -> &'a str {
    arm[key]
        .as_str()
        .unwrap_or_else(|| panic!("arm entry missing string field `{key}`: {arm}"))
}

#[test]
fn registry_has_one_trunk_and_seven_arms() {
    let reg = registry();
    assert_eq!(reg["product"], "CuNi", "the product name is CuNi");
    assert_eq!(
        reg["trunk"]["command"], "cuni check",
        "every arm runs on the same gate"
    );

    let list = arms(&reg);
    assert_eq!(list.len(), 7, "CuNi has exactly seven arms");
    let ids: Vec<&str> = list.iter().map(|a| field(a, "id")).collect();
    assert_eq!(ids, ARM_IDS, "arm order and identity are part of the map");

    for pair in list.windows(2) {
        assert_ne!(
            field(&pair[0], "name"),
            field(&pair[1], "name"),
            "arm names must be unique"
        );
    }
}

#[test]
fn every_arm_points_at_a_real_entry_and_doc() {
    let reg = registry();
    for arm in arms(&reg) {
        let id = field(arm, "id");
        for key in ["path", "entry", "doc"] {
            let rel = field(arm, key);
            assert!(
                root().join(rel).exists(),
                "arm `{id}`: {key} `{rel}` does not exist"
            );
        }
    }

    // The out-of-scope tree must stay out of the product boundary.
    let outside = reg["outside"]
        .as_array()
        .expect("arms.json must list what is outside the product");
    assert!(
        outside
            .iter()
            .any(|o| o["repo"] == "ceedot-rock/cuni-exact"),
        "the private exact tree must be declared outside the product"
    );
}

#[test]
fn human_page_and_readme_name_every_arm() {
    let reg = registry();
    let page = read("ARMS.md");
    assert!(
        page.contains("one product") && page.contains("seven arms"),
        "ARMS.md must state the product shape"
    );
    assert!(
        page.contains("cuni-exact"),
        "ARMS.md must name the workbench that is not an arm"
    );

    for arm in arms(&reg) {
        let name = field(arm, "name");
        let doc = field(arm, "doc");
        assert!(page.contains(name), "ARMS.md does not name the {name} arm");
        assert!(
            page.contains(doc),
            "ARMS.md does not link the {name} arm's doc `{doc}`"
        );
    }

    assert!(
        read("README.md").contains("ARMS.md"),
        "README.md must point at the arm map"
    );
}

#[test]
fn registry_catalog_matches_the_compiler() {
    let reg = registry();
    let listed = Command::new(env!("CARGO_BIN_EXE_cuni"))
        .arg("--list-langs")
        .output()
        .expect("run cuni --list-langs");
    assert!(listed.status.success(), "cuni --list-langs failed");
    let count = String::from_utf8_lossy(&listed.stdout)
        .lines()
        .filter(|l| !l.trim().is_empty())
        .count();
    assert_eq!(
        reg["trunk"]["catalog"].as_u64(),
        Some(count as u64),
        "arms.json catalog count must match the compiler's catalog"
    );
}

#[test]
fn arm_sections_are_not_empty() {
    for arm in arms(&registry()) {
        let surface = field(arm, "surface");
        assert!(
            !surface.trim().is_empty(),
            "arm `{}` needs a one-line surface",
            field(arm, "id")
        );
        assert!(
            Path::new(field(arm, "doc")).extension().is_some(),
            "arm `{}` doc should be a file",
            field(arm, "id")
        );
    }
}
