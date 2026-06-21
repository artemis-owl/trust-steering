package steering.decision

import rego.v1

default allow := false
default rationale := [{"code": "DEFAULT_DENY", "message": "Denied by default."}]

# Helper to check if role matches action dynamically from loaded JSON data
action_allowed_for_role(role, action) if {
    allowed_actions := data.roles[role]
    action == allowed_actions[_]
}

# Allow simple actions if allowed by role
allow if {
    action := input.request.action
    action != "deploy"
    action_allowed_for_role(input.subject.attributes.role, action)
}

# Dev Deploy criteria
allow if {
    input.request.action == "deploy"
    input.request.environment == "dev"
    input.request.attributes.tests_passed == "true"
    action_allowed_for_role(input.subject.attributes.role, "deploy")
}

# Prod Deploy criteria
allow if {
    input.request.action == "deploy"
    input.request.environment == "prod"
    input.request.attributes.human_approved == "true"
    action_allowed_for_role(input.subject.attributes.role, "deploy")
}

# Rationale for simple actions
rationale := [{"code": "ALLOW_ACTION", "message": sprintf("Action '%s' permitted for role '%s'.", [input.request.action, input.subject.attributes.role])}] if {
    action := input.request.action
    action != "deploy"
    action_allowed_for_role(input.subject.attributes.role, action)
}

# Rationale for Dev Deploy
rationale := [{"code": "ALLOW_DEPLOY_DEV", "message": "Dev deploy permitted because tests passed."}] if {
    input.request.action == "deploy"
    input.request.environment == "dev"
    input.request.attributes.tests_passed == "true"
    action_allowed_for_role(input.subject.attributes.role, "deploy")
}

# Rationale for Prod Deploy
rationale := [{"code": "ALLOW_DEPLOY_PROD", "message": "Prod deploy permitted by explicit human approval."}] if {
    input.request.action == "deploy"
    input.request.environment == "prod"
    input.request.attributes.human_approved == "true"
    action_allowed_for_role(input.subject.attributes.role, "deploy")
}

decision := {
    "allow": allow,
    "rationale": rationale
}
