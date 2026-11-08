# Acceptance specification. Automated evidence: tests/integration/broker-flow.mjs
@US06 @TS04
Feature: Read daily water consumption for a unit

  Scenario: Authorized or healthy request
    Given a signed-in user with a current plan can access a unit with no readings in the requested period
    When the client requests GET /api/v1/consumption/daily?unitId=<authorized-unit>
    Then the response status is 200
    And the daily list is empty and the total is zero liters

  Scenario: Boundary or access control
    Given the review environment is available
    When a request for an authorized unit includes an invalid from timestamp
    Then the response status is 400
    And Telemetry rejects the invalid period
