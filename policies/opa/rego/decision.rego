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
    action != "inference"
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

# Inference: the resource is the route (a served model name). The "local"
# route never leaves the machine; any other route is off the machine and is
# permitted only for these declared data classifications. A missing or
# unknown classification is not permitted off the machine.
offsite_classifications := {"public", "internal"}

inference_route_permitted if {
    input.request.resource == "local"
}

inference_route_permitted if {
    input.request.attributes.data_classification in offsite_classifications
}

allow if {
    input.request.action == "inference"
    action_allowed_for_role(input.subject.attributes.role, "inference")
    inference_route_permitted
}

# Rationale for simple actions
rationale := [{"code": "ALLOW_ACTION", "message": sprintf("Action '%s' permitted for role '%s'.", [input.request.action, input.subject.attributes.role])}] if {
    action := input.request.action
    action != "deploy"
    action != "inference"
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

# Rationale for inference on the local route
rationale := [{"code": "ALLOW_INFERENCE_LOCAL", "message": "Inference on the local route permitted; data does not leave the machine."}] if {
    input.request.action == "inference"
    action_allowed_for_role(input.subject.attributes.role, "inference")
    input.request.resource == "local"
}

# Rationale for inference on an off-machine route that the classification permits
rationale := [{"code": "ALLOW_INFERENCE_OFFSITE", "message": sprintf("Inference on route '%s' permitted for data classified '%s'.", [input.request.resource, input.request.attributes.data_classification])}] if {
    input.request.action == "inference"
    action_allowed_for_role(input.subject.attributes.role, "inference")
    input.request.resource != "local"
    inference_route_permitted
}

# Rationale for inference on an off-machine route that the classification forbids
rationale := [{"code": "DENY_INFERENCE_OFFSITE", "message": sprintf("Inference on route '%s' leaves the machine and is not permitted for this data classification (only public or internal may leave); use the local route.", [input.request.resource])}] if {
    input.request.action == "inference"
    action_allowed_for_role(input.subject.attributes.role, "inference")
    input.request.resource != "local"
    not inference_route_permitted
}

decision := {
    "allow": allow,
    "rationale": rationale
}
