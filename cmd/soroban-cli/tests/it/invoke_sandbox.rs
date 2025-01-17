use crate::util::{temp_ledger_file, Sandbox, SorobanCommand, HELLO_WORLD, INVOKER_ACCOUNT_EXISTS};

#[test]
fn source_account_exists() {
    Sandbox::new_cmd()
        .arg("contract")
        .arg("invoke")
        .arg("--ledger-file")
        .arg(temp_ledger_file())
        .arg("--id=1")
        .arg("--wasm")
        .arg(INVOKER_ACCOUNT_EXISTS.path())
        .arg("--fn=invkexists")
        .assert()
        .success()
        .stdout("true\n");
}

#[test]
fn install_wasm_then_deploy_contract() {
    let ledger = temp_ledger_file();
    let hash = HELLO_WORLD.hash();
    Sandbox::new_cmd()
        .arg("contract")
        .arg("install")
        .arg("--ledger-file")
        .arg(&ledger)
        .arg("--wasm")
        .arg(HELLO_WORLD.path())
        .assert()
        .success()
        .stdout(format!("{hash}\n"));

    Sandbox::new_cmd()
        .arg("contract")
        .arg("deploy")
        .arg("--ledger-file")
        .arg(&ledger)
        .arg("--wasm-hash")
        .arg(&format!("{hash}"))
        .arg("--id=1")
        .assert()
        .success()
        .stdout("0000000000000000000000000000000000000000000000000000000000000001\n");
}

#[test]
fn deploy_contract_with_wasm_file() {
    Sandbox::new_cmd()
        .arg("contract")
        .arg("deploy")
        .arg("--ledger-file")
        .arg(temp_ledger_file())
        .arg("--wasm")
        .arg(HELLO_WORLD.path())
        .arg("--id=1")
        .assert()
        .success()
        .stdout("0000000000000000000000000000000000000000000000000000000000000001\n");
}
