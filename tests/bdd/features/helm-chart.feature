Feature: url-shortener Helm chart
  As an operator deploying url-shortener
  I want a production-ready Helm chart published to GHCR
  So that I can install and upgrade the service reliably on Kubernetes

  Background:
    Given the chart at "charts/url-shortener"
    And a Kubernetes cluster at version 1.29 or newer

  Scenario: Chart installs successfully with the minimum required values
    Given a local kind cluster
    When I run:
      """
      helm install url-shortener charts/url-shortener \
        --set app.publicBaseURL=https://short.example \
        --set app.adminBaseURL=https://admin.example \
        --set cassandra.hosts=cassandra:9042 \
        --set session.secret=test-session-secret \
        --wait
      """
    Then the release status is "deployed"
    And the Deployment "url-shortener" becomes Available
    And the pod runs as UID 65532 with a read-only root filesystem

  Scenario: Chart upgrade does not cause Deployment downtime
    Given the chart is already installed at the previous version
    When I run "helm upgrade url-shortener charts/url-shortener --reuse-values --wait"
    Then the upgrade succeeds
    And the Deployment uses a RollingUpdate strategy
    And at least one replica stays Available throughout the rollout

  Scenario: helm lint fails when a required value is missing
    When I run "helm lint charts/url-shortener --set app.publicBaseURL=''"
    Then the command exits non-zero
    And the output mentions "app.publicBaseURL"

  Scenario: helm lint fails when cassandra.hosts is missing
    When I run "helm lint charts/url-shortener --set cassandra.hosts=''"
    Then the command exits non-zero
    And the output mentions "cassandra.hosts"
