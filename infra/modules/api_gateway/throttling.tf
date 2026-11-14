# Stage-wide throttling. It caps how hard the API (and therefore Lambda and the
# database) can be hit. It is shared by all callers: it protects your costs and
# backend, but it does not stop one client from using up the whole allowance.
# Per-IP limits need an AWS WAF rate-based rule.
resource "aws_api_gateway_method_settings" "all" {
  rest_api_id = aws_api_gateway_rest_api.f1_api.id
  stage_name  = aws_api_gateway_stage.prod.stage_name
  method_path = "*/*"

  settings {
    throttling_rate_limit  = 20 # steady requests per second
    throttling_burst_limit = 50 # short bursts
  }
}

# Without this, a throttled request (HTTP 429) carries no CORS headers, and the
# browser reports a confusing CORS error instead of "too many requests".
resource "aws_api_gateway_gateway_response" "throttled_429" {
  rest_api_id   = aws_api_gateway_rest_api.f1_api.id
  response_type = "THROTTLED"
  status_code   = "429"

  response_parameters = {
    "gatewayresponse.header.Access-Control-Allow-Origin"      = "'https://f1.michielve.be'"
    "gatewayresponse.header.Access-Control-Allow-Credentials" = "'true'"
  }
}
