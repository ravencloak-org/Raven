# Session Context

## User Prompts

### Prompt 1

run cr review and create gh issues for them, fix them using parallel agents and create a single PR

### Prompt 2

You are an expert code reviewer. Follow these steps:

      1. If no PR number is provided in the args, run `gh pr list` to show open PRs
      2. If a PR number is provided, run `gh pr view <number>` to get PR details
      3. Run `gh pr diff <number>` to get the diff
      4. Analyze the changes and provide a thorough code review that includes:
         - Overview of what the PR does
         - Analysis of code quality and style
         - Specific suggestions for improvements
         - Any p...

### Prompt 3

failing https://github.com/ravencloak-org/Raven/pull/291/checks?check_run_id=71473459685

### Prompt 4

continue

### Prompt 5

failing https://github.com/ravencloak-org/Raven/pull/291/checks?check_run_id=71473459685

### Prompt 6

failing https://github.com/ravencloak-org/Raven/pull/291/checks?check_run_id=71473459685

### Prompt 7

https://github.REDACTED?pr=299 2026/04/15 14:39:06 OK   00033_payment_intents.sql (5.33ms)
2026/04/15 14:39:06 OK   00034_zitadel_migration.sql (2.45ms)
2026/04/15 14:39:06 goose: successfully migrated database to version: 34
FAIL
coverage: 5.3% of statements in ./internal/...
FAIL    github.com/ravencloak-org/Raven/internal/integration    21.187s
FAIL
Error: Process completed with exit code 1.

### Prompt 8

index everything for our offline tools

### Prompt 9

commit and raise pr if any files changed

### Prompt 10

why are there issues with bug label still open?

### Prompt 11

can supertoken and keycloak both aceess user authroization via spi jar or ldap/kerbos. which is the commong protocol to do this between various products like keycloal, supertokens etc.

### Prompt 12

why are codecov badges still showing unknown and actions failing. was a new release created?

### Prompt 13

fix all of them, zitadel doesn't exist anymore. we replaces it with supertokens.

### Prompt 14

create PR and make sure you are on separate git worktree

### Prompt 15

action failed 59s
Run golangci/golangci-lint-action@v9
Restore cache
Install
run golangci-lint

