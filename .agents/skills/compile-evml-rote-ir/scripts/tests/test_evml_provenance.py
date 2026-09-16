import json
import subprocess
import sys
from pathlib import Path

import pytest

SCRIPT = Path(__file__).resolve().parent.parent / "evml_provenance.py"


def run_script(*argv, cwd=None):
    return subprocess.run(
        [sys.executable, str(SCRIPT), *argv],
        capture_output=True,
        text=True,
        cwd=cwd,
    )


@pytest.fixture
def model_json(tmp_path):
    model = {
        "schema": "evml-model/v1",
        "source": "testdata/fixtures/tiny.evml",
        "sha256": "a" * 64,
        "sections": [
            {"name": "Sales bounded context", "line": 3},
            {"name": "Billing bounded context", "line": 10},
        ],
        "entities": [],
        "frames": [
            {
                "id": "01",
                "kind": "tf",
                "type": "ui",
                "name": "CheckoutScreen",
                "namespace": "",
                "line": 4,
                "section": "",
                "sources": [],
                "dataRef": "",
                "dataType": "",
                "data": "",
            },
            {
                "id": "02",
                "kind": "tf",
                "type": "cmd",
                "name": "PlaceOrder",
                "namespace": "",
                "line": 5,
                "section": "Sales bounded context",
                "sources": ["01"],
                "dataRef": "OrderData",
                "dataType": "",
                "data": "",
            },
            {
                "id": "03",
                "kind": "tf",
                "type": "evt",
                "name": "OrderPlaced",
                "namespace": "",
                "line": 6,
                "section": "Sales bounded context",
                "sources": ["02"],
                "dataRef": "",
                "dataType": "",
                "data": "{ orderId: \"o-1\" }",
            },
            {
                "id": "04",
                "kind": "rf",
                "type": "evt",
                "name": "Sales.OrderPlaced",
                "namespace": "Sales",
                "line": 11,
                "section": "Billing bounded context",
                "sources": [],
                "dataRef": "",
                "dataType": "",
                "data": "",
            },
            {
                "id": "05",
                "kind": "tf",
                "type": "cmd",
                "name": "AuthorizePayment",
                "namespace": "",
                "line": 12,
                "section": "Billing bounded context",
                "sources": ["04"],
                "dataRef": "",
                "dataType": "",
                "data": "",
            },
        ],
        "data": [
            {"name": "OrderData", "dataType": "", "value": "{ cartId: \"c-1\" }", "line": 20}
        ],
        "notes": [
            {"frame": "02", "dataType": "md", "value": "{ note text }", "line": 22}
        ],
        "gwts": [
            {
                "frame": "02",
                "label": "place order",
                "line": 24,
                "given": [
                    {"type": "evt", "name": "CartPriced", "dataType": "", "data": "{ cartId: \"c-1\" }"}
                ],
                "when": [
                    {"type": "cmd", "name": "PlaceOrder", "dataType": "", "data": ""}
                ],
                "then": [
                    {"type": "evt", "name": "OrderPlaced", "dataType": "", "data": ""}
                ],
            }
        ],
    }
    path = tmp_path / "model.json"
    path.write_text(json.dumps(model), encoding="utf-8")
    return path


def write_pipeline(ctx_dir, section_keys):
    ctx_dir.mkdir(parents=True, exist_ok=True)
    nodes = [
        {"id": f"n{i}", "kind": "task", "source": {"section": k}}
        for i, k in enumerate(section_keys)
    ]
    (ctx_dir / "pipeline.yaml").write_text(
        "version: '1.0.0'\nnodes:\n"
        + "".join(
            f"  - id: {n['id']}\n    kind: task\n    source:\n      section: \"{n['source']['section']}\"\n"
            for n in nodes
        ),
        encoding="utf-8",
    )
    (ctx_dir / "compile-report.md").write_text("# report\n", encoding="utf-8")
    (ctx_dir / "eval.yaml").write_text("cases: []\n", encoding="utf-8")


def write_prov(model_json, ctx_dir, model="tiny", context="sales",
               section_name="Sales bounded context", frames="02,03"):
    return run_script(
        "write", str(model_json),
        "--model", model,
        "--context", context,
        "--section-name", section_name,
        "--frames", frames,
        "--pipeline", str(ctx_dir / "pipeline.yaml"),
        "--out", str(ctx_dir / "provenance.json"),
        "--compiled-at", "2026-01-01",
    )


def test_hash_keys_and_line_exclusion(model_json):
    res = run_script("hash", str(model_json))
    assert res.returncode == 0, res.stderr
    out = json.loads(res.stdout)
    assert out["__file__"] == "a" * 64
    assert "tf 02 cmd PlaceOrder" in out
    assert "rf 04 evt Sales.OrderPlaced" in out
    # reformatting (line/section changes) must not churn hashes
    model = json.loads(model_json.read_text())
    before = out["tf 02 cmd PlaceOrder"]
    model["frames"][1]["line"] = 999
    model["frames"][1]["section"] = "Renamed"
    model["gwts"][0]["line"] = 555
    model_json.write_text(json.dumps(model))
    out2 = json.loads(run_script("hash", str(model_json)).stdout)
    assert out2["tf 02 cmd PlaceOrder"] == before


def test_write_success(model_json, tmp_path):
    ctx = tmp_path / "COMPILED" / "tiny" / "sales"
    write_pipeline(ctx, ["tf 02 cmd PlaceOrder", "tf 03 evt OrderPlaced"])
    res = write_prov(model_json, ctx)
    assert res.returncode == 0, res.stderr
    prov = json.loads((ctx / "provenance.json").read_text())
    assert prov["schema"] == "evml-rote-ir-provenance/v1"
    assert prov["frames"] == ["02", "03"]
    assert prov["pipeline_version"] == "1.0.0"
    assert set(prov["sections"]) == {"tf 02 cmd PlaceOrder", "tf 03 evt OrderPlaced"}
    assert prov["nodes"]["n0"]["section"] == "tf 02 cmd PlaceOrder"


def test_write_rejects_bad_source_section(model_json, tmp_path):
    ctx = tmp_path / "COMPILED" / "tiny" / "sales"
    write_pipeline(ctx, ["tf 02 cmd PlaceOrder", "tf 99 evt Bogus"])
    res = write_prov(model_json, ctx)
    assert res.returncode == 1
    assert "n1" in res.stderr


def test_write_rejects_unknown_frame(model_json, tmp_path):
    ctx = tmp_path / "COMPILED" / "tiny" / "sales"
    write_pipeline(ctx, ["tf 02 cmd PlaceOrder"])
    res = write_prov(model_json, ctx, frames="02,99")
    assert res.returncode == 1
    assert "99" in res.stderr


def _compiled(tmp_path, model_json):
    ctx = tmp_path / "COMPILED" / "tiny" / "sales"
    write_pipeline(ctx, ["tf 02 cmd PlaceOrder", "tf 03 evt OrderPlaced"])
    res = write_prov(model_json, ctx)
    assert res.returncode == 0, res.stderr
    return tmp_path / "COMPILED" / "tiny"


def test_check_current(model_json, tmp_path):
    compiled = _compiled(tmp_path, model_json)
    res = run_script("check", str(model_json), "--compiled", str(compiled), "--json")
    assert res.returncode == 0, res.stderr
    report = json.loads(res.stdout)
    assert report["contexts"][0]["status"] == "current"
    assert report["contexts"][0]["preserved_nodes"] == ["n0", "n1"]
    assert set(report["unassigned"]) == {"01", "04", "05"}


def test_check_stale_changed(model_json, tmp_path):
    compiled = _compiled(tmp_path, model_json)
    model = json.loads(model_json.read_text())
    model["gwts"][0]["then"][0]["data"] = "{ mutated: true }"
    model_json.write_text(json.dumps(model))
    res = run_script("check", str(model_json), "--compiled", str(compiled), "--json")
    assert res.returncode == 0
    report = json.loads(res.stdout)
    ctx = report["contexts"][0]
    assert ctx["status"] == "stale"
    assert ctx["reasons"]["changed"] == ["tf 02 cmd PlaceOrder"]
    assert ctx["stale_nodes"] == ["n0"]
    assert ctx["preserved_nodes"] == ["n1"]


def test_check_added_frame(model_json, tmp_path):
    compiled = _compiled(tmp_path, model_json)
    model = json.loads(model_json.read_text())
    new_frame = dict(model["frames"][2])
    new_frame["id"] = "06"
    new_frame["name"] = "OrderEmailSent"
    model["frames"].append(new_frame)
    model_json.write_text(json.dumps(model))
    res = run_script("check", str(model_json), "--compiled", str(compiled), "--json")
    report = json.loads(res.stdout)
    assert report["contexts"][0]["reasons"]["added"] == ["06"]


def test_check_missing_artifact(model_json, tmp_path):
    compiled = _compiled(tmp_path, model_json)
    (compiled / "sales" / "eval.yaml").unlink()
    res = run_script("check", str(model_json), "--compiled", str(compiled), "--json")
    report = json.loads(res.stdout)
    ctx = report["contexts"][0]
    assert ctx["status"] == "stale"
    assert "eval.yaml" in ctx["reasons"]["missing_artifacts"]


def test_check_missing_pipeline(model_json, tmp_path):
    compiled = _compiled(tmp_path, model_json)
    (compiled / "sales" / "pipeline.yaml").unlink()
    res = run_script("check", str(model_json), "--compiled", str(compiled), "--json")
    report = json.loads(res.stdout)
    assert report["contexts"][0]["status"] == "missing"


def test_check_human_output(model_json, tmp_path):
    compiled = _compiled(tmp_path, model_json)
    res = run_script("check", str(model_json), "--compiled", str(compiled))
    assert res.returncode == 0
    assert "context" in res.stdout and "status" in res.stdout
    assert "sales" in res.stdout


def test_write_rejects_missing_node_id(model_json, tmp_path):
    ctx = tmp_path / "COMPILED" / "tiny" / "sales"
    ctx.mkdir(parents=True, exist_ok=True)
    (ctx / "pipeline.yaml").write_text(
        "version: '1.0.0'\nnodes:\n"
        "  - kind: task\n    source:\n      section: \"tf 02 cmd PlaceOrder\"\n",
        encoding="utf-8",
    )
    res = write_prov(model_json, ctx)
    assert res.returncode == 1
    assert "no required 'id'" in res.stderr


def test_check_missing_eval_set(model_json, tmp_path):
    ctx = tmp_path / "COMPILED" / "tiny" / "sales"
    ctx.mkdir(parents=True, exist_ok=True)
    (ctx / "pipeline.yaml").write_text(
        "version: '1.0.0'\nnodes:\n"
        "  - id: n0\n    kind: llm_judge\n    source:\n      section: \"tf 02 cmd PlaceOrder\"\n"
        "    eval_set: \"evals/vet_contact.jsonl\"\n",
        encoding="utf-8",
    )
    (ctx / "compile-report.md").write_text("# report\n", encoding="utf-8")
    (ctx / "eval.yaml").write_text("cases: []\n", encoding="utf-8")
    (ctx / "evals").mkdir(exist_ok=True)
    (ctx / "evals" / "vet_contact.jsonl").write_text('{"prompt": "x"}\n', encoding="utf-8")
    res = write_prov(model_json, ctx)
    assert res.returncode == 0, res.stderr
    (ctx / "evals" / "vet_contact.jsonl").unlink()
    res = run_script("check", str(model_json), "--compiled", str(tmp_path / "COMPILED" / "tiny"), "--json")
    assert res.returncode == 0, res.stderr
    report = json.loads(res.stdout)
    c = report["contexts"][0]
    assert c["status"] == "stale"
    assert "evals/vet_contact.jsonl" in c["reasons"]["missing_artifacts"]
