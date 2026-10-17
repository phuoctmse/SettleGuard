import pytest

from check_report import evaluate, load_accepted


def _report(*alerts):
    return {"site": [{"@name": "http://localhost:8000", "alerts": list(alerts)}]}


def _alert(pluginid, riskcode, name="x"):
    return {"pluginid": str(pluginid), "riskcode": str(riskcode), "name": name, "instances": [{"uri": "http://localhost:8000/ledger/x"}]}


def test_high_alert_not_accepted_fails():
    high, _ = evaluate([_report(_alert(40018, 3, "SQL Injection"))], [])
    assert [a["pluginid"] for a in high] == ["40018"]


def test_accepted_high_alert_passes():
    accepted = [{"plugin_id": "40018", "finding_id": "SG-PT-99", "reason": "test"}]
    high, _ = evaluate([_report(_alert(40018, 3))], accepted)
    assert high == []


def test_medium_is_reported_not_failed():
    high, medium = evaluate([_report(_alert(10021, 2))], [])
    assert high == []
    assert [a["pluginid"] for a in medium] == ["10021"]


@pytest.mark.parametrize("missing", ["plugin_id", "finding_id", "reason"])
def test_accepted_entry_missing_a_field_is_rejected(tmp_path, missing):
    entry = {"plugin_id": "1", "finding_id": "SG-PT-01", "reason": "r"}
    del entry[missing]
    path = tmp_path / "accepted.yaml"
    path.write_text(f"accepted: [{entry}]".replace("'", '"'))
    with pytest.raises(ValueError, match=missing):
        load_accepted(path)


def test_empty_accepted_file_is_allowed(tmp_path):
    path = tmp_path / "accepted.yaml"
    path.write_text("accepted: []\n")
    assert load_accepted(path) == []


def test_scan_with_no_results_fails_closed(tmp_path, capsys):
    from check_report import main

    accepted = tmp_path / "accepted.yaml"
    accepted.write_text("accepted: []\n", encoding="utf-8")
    report = tmp_path / "empty.json"
    report.write_text("{}", encoding="utf-8")
    assert main(["check_report.py", str(accepted), str(report)]) == 1
    assert "failing closed" in capsys.readouterr().out


def test_one_empty_report_among_good_ones_fails_closed(tmp_path):
    import json

    from check_report import main

    accepted = tmp_path / "accepted.yaml"
    accepted.write_text("accepted: []\n", encoding="utf-8")
    good = tmp_path / "good.json"
    good.write_text(json.dumps(_report()), encoding="utf-8")
    bad = tmp_path / "bad.json"
    bad.write_text("{}", encoding="utf-8")
    assert main(["check_report.py", str(accepted), str(good), str(bad)]) == 1
