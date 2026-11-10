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

# Inference: the resource is the route; "local" stays on the machine, any
# other route is off the machine and needs a public or internal classification.

test_inference_confidential_offsite_denied if {
  d := decision with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "inference", "resource": "claude-sonnet", "environment": "dev", "attributes": {"data_classification": "confidential"}}
  }
  not d.allow
  d.rationale[0].code == "DENY_INFERENCE_OFFSITE"
}

test_inference_confidential_local_allowed if {
  d := decision with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "inference", "resource": "local", "environment": "dev", "attributes": {"data_classification": "confidential"}}
  }
  d.allow
  d.rationale[0].code == "ALLOW_INFERENCE_LOCAL"
}

test_inference_internal_offsite_allowed if {
  d := decision with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "inference", "resource": "claude-sonnet", "environment": "dev", "attributes": {"data_classification": "internal"}}
  }
  d.allow
  d.rationale[0].code == "ALLOW_INFERENCE_OFFSITE"
}

test_inference_unclassified_offsite_denied if {
  d := decision with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "inference", "resource": "claude-sonnet", "environment": "dev", "attributes": {}}
  }
  not d.allow
  d.rationale[0].code == "DENY_INFERENCE_OFFSITE"
}

test_inference_unknown_classification_offsite_denied if {
  d := decision with input as {
    "subject": {"id": "user:alice", "attributes": {"role": "developer"}},
    "request": {"action": "inference", "resource": "claude-sonnet", "environment": "dev", "attributes": {"data_classification": "secret-ish"}}
  }
  not d.allow
  d.rationale[0].code == "DENY_INFERENCE_OFFSITE"
}

test_inference_role_without_it_denied if {
  d := decision with input as {
    "subject": {"id": "user:dave", "attributes": {"role": "reviewer"}},
    "request": {"action": "inference", "resource": "local", "environment": "dev", "attributes": {"data_classification": "public"}}
  }
  not d.allow
  d.rationale[0].code == "DEFAULT_DENY"
}
