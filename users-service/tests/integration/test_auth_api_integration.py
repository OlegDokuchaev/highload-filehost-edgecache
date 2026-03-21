import pytest
from fastapi.testclient import TestClient


pytestmark = pytest.mark.integration


def test_register_login_verify_flow(test_client: TestClient) -> None:
    register_response = test_client.post(
        "/auth/register",
        json={"login": "MyLogin", "password": "VeryStrongPass!1"},
    )
    assert register_response.status_code == 201
    body = register_response.json()
    assert body["normalized_login"] == "mylogin"
    assert body["user_id"]

    login_response = test_client.post(
        "/auth/login",
        json={"login": "MyLogin", "password": "VeryStrongPass!1"},
    )
    assert login_response.status_code == 200
    login_body = login_response.json()
    assert login_body["access_token"]
    assert login_body["normalized_login"] == "mylogin"

    verify_response = test_client.post(
        "/auth/verify",
        json={"token": login_body["access_token"]},
    )
    assert verify_response.status_code == 200
    verify_body = verify_response.json()
    assert verify_body["active"] is True
    assert verify_body["normalized_login"] == "mylogin"


def test_register_conflict(test_client: TestClient) -> None:
    payload = {"login": "SameLogin", "password": "VeryStrongPass!1"}
    first = test_client.post("/auth/register", json=payload)
    second = test_client.post("/auth/register", json=payload)

    assert first.status_code == 201
    assert second.status_code == 409


def test_login_invalid_credentials(test_client: TestClient) -> None:
    test_client.post(
        "/auth/register",
        json={"login": "AnotherLogin", "password": "VeryStrongPass!1"},
    )
    response = test_client.post(
        "/auth/login",
        json={"login": "AnotherLogin", "password": "wrong-password"},
    )
    assert response.status_code == 401


def test_verify_invalid_token_returns_active_false(test_client: TestClient) -> None:
    response = test_client.post("/auth/verify", json={"token": "not-a-jwt"})
    assert response.status_code == 200
    assert response.json() == {"active": False, "user_id": None, "normalized_login": None}
