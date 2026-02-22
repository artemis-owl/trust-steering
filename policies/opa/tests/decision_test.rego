package steering.decision

test_allow_dev_developer_deploy if {
  decision.allow with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "deploy", "resource": "service:gateway", "environment": "dev", "attributes": {}}
  }
}

test_deny_prod_even_for_developer if {
  not decision.allow with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "deploy", "resource": "service:gateway", "environment": "prod", "attributes": {}}
  }
}

