package steering.decision

default decision := {"allow": false, "rationale": [{"code": "DEFAULT_DENY", "message": "Denied by default."}]}

# Minimal input shape (evolves later):
# input.subject: {id, attributes: {role, ...}}
# input.request: {action, resource, environment, attributes: {...}}

decision := {"allow": true, "rationale": [{"code": "ALLOW_DEV", "message": "Dev actions allowed for developers."}]} if {
  input.request.environment == "dev"
  input.subject.attributes.role == "developer"
  allowed_action[input.request.action]
}

allowed_action["deploy"]
allowed_action["read"]

