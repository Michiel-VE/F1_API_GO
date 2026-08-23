output "lambda_role_arn" {
  value = aws_iam_role.lambda_role.arn
}

output "deploy_role_arn" {
  value = aws_iam_role.github_deploy.arn
}