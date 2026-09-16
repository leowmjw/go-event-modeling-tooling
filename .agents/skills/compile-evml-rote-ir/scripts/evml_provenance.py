# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml"]
# ///
"""Provenance helper for the compile-evml-rote-ir skill.

Tracks which source frames (with their gwt/data/note blocks) each compiled
bounded-context pipeline.yaml was derived from, so stale slices can be
recompiled without re-deriving unchanged ones.
"""

import argparse
import datetime
import hashlib
import json
import sys
from pathlib import Path

import yaml

REQUIRED_ARTIFACTS = ["pipeline.yaml", "compile-report.md", "eval.yaml", "provenance.json"]
IR_SCHEMA_REF = "ROTE/rote/skills/rote-compile/references/ir-schema.md"


def load_model(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def section_key(frame):
    return f"{frame['kind']} {frame['id']} {frame['type']} {frame['name']}"


def _strip(d, *keys):
    return {k: v for k, v in d.items() if k not in keys}


def frame_content(model, frame):
    """Canonical, line-number-free content for one frame section."""
    frame_id = frame["id"]
    gwts = [_strip(g, "line") for g in model.get("gwts", []) if g["frame"] == frame_id]
    notes = [_strip(n, "line") for n in model.get("notes", []) if n["frame"] == frame_id]
    hotspots = [
        _strip(h, "line") for h in model.get("hotspots", []) if h["frame"] == frame_id
    ]
    data = None
    data_ref = frame.get("dataRef") or ""
    if data_ref:
        for d in model.get("data", []):
            if d["name"] == data_ref:
                data = _strip(d, "line")
                break
    return {
        "frame": _strip(frame, "line", "section"),
        "gwts": gwts,
        "data": data,
        "notes": notes,
        "hotspots": hotspots,
    }


def content_hash(obj):
    canonical = json.dumps(obj, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def section_hashes(model):
    """Map of section key -> sha256 for every frame in the model."""
    return {section_key(f): content_hash(frame_content(model, f)) for f in model.get("frames", [])}


def cmd_hash(args):
    model = load_model(args.model_json)
    out = section_hashes(model)
    out["__file__"] = model.get("sha256", "")
    print(json.dumps(out, indent=2, sort_keys=True))
    return 0


def _pipeline_nodes(pipeline):
    nodes = pipeline.get("nodes", []) or []
    if isinstance(nodes, dict):
        return [{"id": k, **(v or {})} for k, v in nodes.items()]
    return [n for n in nodes if isinstance(n, dict)]


def _node_id(node, index):
    nid = node.get("id")
    if nid is None:
        raise ValueError(f"node at index {index} has no required 'id' field")
    return str(nid)


def cmd_write(args):
    model = load_model(args.model_json)
    frames_arg = [f.strip() for f in args.frames.split(",") if f.strip()]
    by_id = {f["id"]: f for f in model.get("frames", [])}
    missing = [fid for fid in frames_arg if fid not in by_id]
    if missing:
        print(f"error: frame ids not in model: {', '.join(missing)}", file=sys.stderr)
        return 1
    hashes = section_hashes(model)
    wanted_keys = {}
    for fid in frames_arg:
        key = section_key(by_id[fid])
        wanted_keys[key] = hashes[key]

    pipeline_path = Path(args.pipeline)
    with open(pipeline_path, "r", encoding="utf-8") as f:
        pipeline = yaml.safe_load(f) or {}

    bad_nodes = []
    missing_ids = []
    nodes_map = {}
    for i, node in enumerate(_pipeline_nodes(pipeline)):
        try:
            nid = _node_id(node, i)
        except ValueError as e:
            missing_ids.append(str(e))
            continue
        section = (node.get("source") or {}).get("section")
        if section not in wanted_keys:
            bad_nodes.append(nid)
            continue
        nodes_map[nid] = {"section": section, "content_hash": wanted_keys[section]}
    if missing_ids:
        print("error: " + "; ".join(missing_ids), file=sys.stderr)
        return 1
    if bad_nodes:
        print(
            "error: nodes with missing or out-of-scope source.section: " + ", ".join(bad_nodes),
            file=sys.stderr,
        )
        return 1

    prov = {
        "schema": "evml-rote-ir-provenance/v1",
        "source_model": model.get("source", ""),
        "source_sha256": model.get("sha256", ""),
        "model": args.model,
        "context": args.context,
        "section_name": args.section_name,
        "frames": frames_arg,
        "ir": "pipeline.yaml",
        "ir_schema_ref": IR_SCHEMA_REF,
        "compiled_at": args.compiled_at or datetime.date.today().isoformat(),
        "pipeline_version": str(pipeline.get("version", "")),
        "sections": wanted_keys,
        "nodes": nodes_map,
    }
    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(prov, f, indent=2, sort_keys=False)
        f.write("\n")
    return 0


def _referenced_files(pipeline):
    """impl:/signature:/eval_set: file paths referenced by nodes, relative to context dir."""
    refs = []
    for node in _pipeline_nodes(pipeline):
        for field in ("impl", "signature", "eval_set"):
            value = node.get(field)
            if isinstance(value, str) and ":" in value and field in ("impl", "signature"):
                refs.append(value.split(":", 1)[0])
            elif isinstance(value, str) and value:
                refs.append(value)
    return refs


def _check_context(ctx_dir, model, hashes):
    prov_path = ctx_dir / "provenance.json"
    with open(prov_path, "r", encoding="utf-8") as f:
        prov = json.load(f)
    result = {
        "context": ctx_dir.name,
        "status": "current",
        "reasons": {},
        "preserved_nodes": [],
        "stale_nodes": [],
        "frames": prov.get("frames", []),
    }
    reasons = result["reasons"]

    pipeline_path = ctx_dir / "pipeline.yaml"
    if not pipeline_path.exists():
        result["status"] = "missing"
        reasons.setdefault("missing_artifacts", []).append("pipeline.yaml")
        # still record node freshness below where possible
    pipeline = None
    if pipeline_path.exists():
        with open(pipeline_path, "r", encoding="utf-8") as f:
            pipeline = yaml.safe_load(f) or {}

    missing_artifacts = [a for a in REQUIRED_ARTIFACTS if not (ctx_dir / a).exists()]
    if pipeline is not None:
        for ref in _referenced_files(pipeline):
            if not (ctx_dir / ref).exists():
                missing_artifacts.append(ref)
    if missing_artifacts:
        reasons["missing_artifacts"] = sorted(set(missing_artifacts))

    model_frames = {f["id"]: f for f in model.get("frames", [])}
    recorded_sections = prov.get("sections", {})
    changed = [k for k, sha in recorded_sections.items() if hashes.get(k) != sha]
    if changed:
        reasons["changed"] = sorted(changed)
    removed = [fid for fid in prov.get("frames", []) if fid not in model_frames]
    if removed:
        reasons["removed"] = sorted(removed)
    section_name = prov.get("section_name") or ""
    if section_name:
        added = [
            f["id"]
            for f in model.get("frames", [])
            if f.get("section") == section_name and f["id"] not in prov.get("frames", [])
        ]
        if added:
            reasons["added"] = sorted(added)

    for nid, info in (prov.get("nodes") or {}).items():
        current = hashes.get(info.get("section"))
        if current is not None and current == info.get("content_hash"):
            result["preserved_nodes"].append(nid)
        else:
            result["stale_nodes"].append(nid)
    result["preserved_nodes"].sort()
    result["stale_nodes"].sort()

    if reasons:
        result["status"] = "stale"
        if not pipeline_path.exists():
            result["status"] = "missing"
    return result


def cmd_check(args):
    model = load_model(args.model_json)
    hashes = section_hashes(model)
    compiled = Path(args.compiled)
    if not compiled.is_dir():
        print(f"error: compiled dir not found: {compiled}", file=sys.stderr)
        return 1

    results = []
    assigned = set()
    for child in sorted(compiled.iterdir()):
        if not child.is_dir():
            continue
        if not (child / "provenance.json").exists():
            continue
        try:
            res = _check_context(child, model, hashes)
        except (OSError, json.JSONDecodeError, yaml.YAMLError) as e:
            print(f"error: {child}: {e}", file=sys.stderr)
            return 1
        assigned.update(res["frames"])
        results.append(res)

    unassigned = sorted(
        f["id"] for f in model.get("frames", []) if f["id"] not in assigned
    )
    report = {"contexts": results, "unassigned": unassigned}

    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print(f"{'context':<24} {'status':<8} reasons")
        print(f"{'-' * 24} {'-' * 8} {'-' * 40}")
        for r in results:
            reason_strs = []
            for kind, items in r["reasons"].items():
                reason_strs.append(f"{kind}: {', '.join(items)}")
            print(f"{r['context']:<24} {r['status']:<8} {'; '.join(reason_strs)}")
        if unassigned:
            print("\nunassigned frames: " + ", ".join(unassigned))
    return 0


def main(argv=None):
    ap = argparse.ArgumentParser(prog="evml_provenance")
    sub = ap.add_subparsers(dest="cmd", required=True)

    p_hash = sub.add_parser("hash", help="print per-frame-section sha256 map")
    p_hash.add_argument("model_json")
    p_hash.set_defaults(func=cmd_hash)

    p_write = sub.add_parser("write", help="write provenance.json for a compiled context")
    p_write.add_argument("model_json")
    p_write.add_argument("--model", required=True)
    p_write.add_argument("--context", required=True)
    p_write.add_argument("--section-name", required=True)
    p_write.add_argument("--frames", required=True, help="comma-separated frame ids")
    p_write.add_argument("--pipeline", required=True)
    p_write.add_argument("--out", required=True)
    p_write.add_argument("--compiled-at", default=None)
    p_write.set_defaults(func=cmd_write)

    p_check = sub.add_parser("check", help="classify compiled contexts as current/stale/missing")
    p_check.add_argument("model_json")
    p_check.add_argument("--compiled", required=True)
    p_check.add_argument("--json", action="store_true")
    p_check.set_defaults(func=cmd_check)

    args = ap.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
