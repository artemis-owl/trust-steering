package steering.decision

test_allow_dev_developer_deploy if {
  decision.allow with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "deploy", "resource": "service:gateway", "environment": "dev", "attributes": {"tests_passed": "true"}}
  }
}

test_deny_prod_even_for_developer if {
  not decision.allow with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "deploy", "resource": "service:gateway", "environment": "prod", "attributes": {}}
  }
}

test_allow_designer_build if {
  decision.allow with input as {
    "subject": {"id": "user:bob", "attributes": {"role": "architect"}},
    "request": {"action": "build", "resource": "repo:game", "environment": "dev", "attributes": {}}
  }
}

test_deny_designer_deploy if {
  not decision.allow with input as {
    "subject": {"id": "user:bob", "attributes": {"role": "architect"}},
    "request": {"action": "deploy", "resource": "service:game", "environment": "dev", "attributes": {"tests_passed": "true"}}
  }
}

test_allow_engineer_refactor if {
  decision.allow with input as {
    "subject": {"id": "user:charlie", "attributes": {"role": "engineer"}},
    "request": {"action": "refactor", "resource": "repo:game", "environment": "dev", "attributes": {}}
  }
}

test_allow_engineer_deploy_dev if {
  decision.allow with input as {
    "subject": {"id": "user:charlie", "attributes": {"role": "engineer"}},
    "request": {"action": "deploy", "resource": "service:game", "environment": "dev", "attributes": {"tests_passed": "true"}}
  }
}

test_deny_engineer_deploy_dev_without_tests if {
  not decision.allow with input as {
    "subject": {"id": "user:charlie", "attributes": {"role": "engineer"}},
    "request": {"action": "deploy", "resource": "service:game", "environment": "dev", "attributes": {"tests_passed": "false"}}
  }
}

test_allow_reviewer_build if {
  decision.allow with input as {
    "subject": {"id": "user:dave", "attributes": {"role": "reviewer"}},
    "request": {"action": "build", "resource": "repo:game", "environment": "dev", "attributes": {}}
  }
}

test_deny_reviewer_refactor if {
  not decision.allow with input as {
    "subject": {"id": "user:dave", "attributes": {"role": "reviewer"}},
    "request": {"action": "refactor", "resource": "repo:game", "environment": "dev", "attributes": {}}
  }
}

test_designer_plan if {
  decision.allow with input as {
    "subject": {"id": "user:bob", "attributes": {"role": "architect"}},
    "request": {"action": "plan", "resource": "repo:game", "environment": "dev", "attributes": {}}
  }
}

test_reviewer_review if {
  decision.allow with input as {
    "subject": {"id": "user:dave", "attributes": {"role": "reviewer"}},
    "request": {"action": "review", "resource": "repo:game", "environment": "dev", "attributes": {}}
  }
}
