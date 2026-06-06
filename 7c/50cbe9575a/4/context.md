# Session Context

## User Prompts

### Prompt 1

what was the status of deployment, use ssh root@64.176.97.248 to deploy in prod.

### Prompt 2

continue

### Prompt 3

<task-notification>
<task-id>bx3mmrgsc</task-id>
<tool-use-id>toolu_015XeDaGYggSuGRAN582ciiN</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Playwright with /raven base" completed (exit code 0)</summary>
</task-notification>

### Prompt 4

<task-notification>
<task-id>bmx3j3dhr</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Playwright /raven take 2 — config inside frontend" completed (exit code 0)</summary>
</task-notification>

### Prompt 5

[Request interrupted by user]

### Prompt 6

it's merged, what next..check if it's live and working

### Prompt 7

it's merged, what next..check if it's live and working

### Prompt 8

<task-notification>
<task-id>bfvea9ciy</task-id>
<tool-use-id>toolu_01VyPDEMovngad15gc96ScjS</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Build + push frontend:raven-prefixed (retry, Docker now running)" completed (exit code 0)</summary>
</task-notification>

### Prompt 9

done

### Prompt 10

https://github.REDACTED failing

### Prompt 11

Check status of Release + Demo deploy workflows for tag v0.4.0, and verify whether https://demo.ravencloak.org/raven/api/v1/config now returns valid JSON (proves PR #626 route fix landed). Report status under 300 words. If demo-deploy completed and API works but frontend assets still 404, remind user about the FRONTEND_IMAGE override SSM command (Task #15).

### Prompt 12

it's merged

### Prompt 13

<task-notification>
<task-id>bmhlvpw97</task-id>
<tool-use-id>toolu_01Y6Uq7d1E5kroRnPkeYGrsc</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch Release v0.4.1 to completion" completed (exit code 0)</summary>
</task-notification>

### Prompt 14

https://github.REDACTED https://github.REDACTED

### Prompt 15

<task-notification>
<task-id>byma6t41d</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch Release v0.4.1 retry to completion" completed (exit code 0)</summary>
</task-notification>

### Prompt 16

<task-notification>
<task-id>btsy67xqd</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch retry to completion" completed (exit code 0)</summary>
</task-notification>

### Prompt 17

it merged, checked if release and deployment happened?

### Prompt 18

<task-notification>
<task-id>btkw3skz9</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch the workflow_dispatch demo-deploy run" completed (exit code 0)</summary>
</task-notification>

### Prompt 19

done that

### Prompt 20

<task-notification>
<task-id>bamcdxwyw</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch the re-run" completed (exit code 0)</summary>
</task-notification>

### Prompt 21

raven git:(feat/demo-deploy-vultr-cutover) ✗ ssh -i ~/.ssh/id_ed25739 root@64.176.97.248 'ls -la /opt/raven /etc/cloudflared && docker --version && docker compose version'
Warning: Identity file /Users/jobinlawrance/.ssh/id_ed25739 not accessible: No such file or directory.
/etc/cloudflared:
total 16
drwx------   2 root root 4096 May 19 10:27 .
drwxr-xr-x 116 root root 4096 May 19 17:11 ..
-rw-------   1 root root  161 May 19 10:27 ac5bd284-c908-4c7d-8666-f6e8e824c693.json
-rw-r--r--   1 root ...

### Prompt 22

<task-notification>
<task-id>bgsruhnt9</task-id>
<tool-use-id>toolu_01Wk6Ky2K7ZE6TWGNkkGPR6S</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch run 26113205294" completed (exit code 0)</summary>
</task-notification>

### Prompt 23

<task-notification>
<task-id>bbefb2amt</task-id>
<tool-use-id>toolu_01JurDqdm6EfmjdKe4WhgaKo</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch run 26113755746 (tag=main)" completed (exit code 0)</summary>
</task-notification>

### Prompt 24

what to add then, give specific commands to run

### Prompt 25

why can't you ssh and do it yourself

### Prompt 26

what's happening?

### Prompt 27

https://vultr-demo.ravencloak.org/raven/api/v1/config return DNS_PROBE_FINISHED_NXDOMAIN

### Prompt 28

[Image #1] Let’s remove this section above footer on both versions

### Prompt 29

[Image: source: /Users/jobinlawrance/.claude/image-cache/337cea8f-5a6b-4199-bc68-1de07b76ec01/1.png]

### Prompt 30

done, check

### Prompt 31

even forget the proxy, give me whtever link that work

### Prompt 32

<task-notification>
<task-id>bzoo0whu0</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Build + push frontend:raven-self-contained with public IP URLs + nginx reverse-proxy config" completed (exit code 0)</summary>
</task-notification>

### Prompt 33

<task-notification>
<task-id>bisuvc2bu</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Rebuild frontend with corrected nginx.conf (proxy before rewrite)" completed (exit code 0)</summary>
</task-notification>

### Prompt 34

did you open the port on vultr?

### Prompt 35

done, no more public port. added ssh only firewall

### Prompt 36

yes do that

### Prompt 37

yes draft it

### Prompt 38

yes let's start the followups using parallel agents

### Prompt 39

Base directory for this skill: /Users/jobinlawrance/.claude/plugins/cache/claude-plugins-official/superpowers/5.1.0/skills/dispatching-parallel-agents

# Dispatching Parallel Agents

## Overview

You delegate tasks to specialized agents with isolated context. By precisely crafting their instructions and context, you ensure they stay focused and succeed at their task. They should never inherit your session's context or history — you construct exactly what they need. This also preserves your own...

### Prompt 40

<task-notification>
<task-id>a54f5078a745c4043</task-id>
<tool-use-id>toolu_0158yUm9dY8FQn5Gs9sDntX2</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Agent "Stand up second cloudflared tunnel" completed</summary>
<result>## Phase 2 complete — all green

| Check | Result |
|---|---|
| `cloudflared-observability.service` is-active | `active` |...

### Prompt 41

let's get the whole raven stack including demo up and running along with the cloud on Vultr, forget aws

### Prompt 42

<task-notification>
<task-id>a54f5078a745c4043</task-id>
<tool-use-id>toolu_01CXy38TD777vQDYDgAxPa1X</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Agent "Stand up second cloudflared tunnel" completed</summary>
<result>## Phase 3 — DNS flipped, but `/raven/api/v1/config` returned 404

| Check | Result |
|---|---|
| `cloudflared-demo` is-ac...

### Prompt 43

done

### Prompt 44

let's set up the actual google login

### Prompt 45

done, 371751291825-vmdpo4gb3nmtrdt8979lobcqg137r5rh.apps.googleusercontent.com
client id

### Prompt 46

➜  raven git:(feat/auth-path-prefix-google-oauth) ✗    read -rs -p 'paste Google Client Secret + Enter: ' GS && echo && \
  ssh -i ~/.ssh/id_ed25519 root@64.176.97.248 \
    "cd /opt/raven && \
     cp .env.server .env.server.bak.\$(date -u +%Y%m%dT%H%M%SZ) && \
     sed -i 's|^GOOGLE_CLIENT_ID=.*|REDACTED.apps.googleusercontent.com|' .env.server && \
     sed -i 's|^GOOGLE_CLIENT_SECRET=.*|GOOGLE_CLIENT_SECRET=$GS|' .env.server && \
    ...

### Prompt 47

paste Google Client Secret + Enter:
=== verify (values redacted) ===
GOOGLE_CLIENT_ID=…
GOOGLE_CLIENT_SECRET=…
=== length sanity check ===
secret length: 35

### Prompt 48

<task-notification>
<task-id>bvu3hs9pj</task-id>
<tool-use-id>toolu_01KqPZDtDHJqQmj6u2oDpQz9</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch docker.yml run to publish completion" completed (exit code 0)</summary>
</task-notification>

### Prompt 49

Welcome back
Sign in to your account to continue

Sign in with Google
Failed to fetch

### Prompt 50

use the chrome extension to test yourself

### Prompt 51

<task-notification>
<task-id>bkx5h0jax</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Ad-hoc local rebuild of frontend:raven-self-contained with the auth-base-path fix" completed (exit code 0)</summary>
</task-notification>

### Prompt 52

same issue

### Prompt 53

[Image #3]

### Prompt 54

[Image: source: /Users/jobinlawrance/.claude/image-cache/337cea8f-5a6b-4199-bc68-1de07b76ec01/3.png]

### Prompt 55

it was always this values since yesterday

### Prompt 56

https://accounts.google.com/signin/oauth/error?REDACTED...

### Prompt 57

<task-notification>
<task-id>bu72r1osz</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Rebuild + push + deploy frontend with /raven/callback fix" completed (exit code 0)</summary>
</task-notification>

### Prompt 58

check now

### Prompt 59

still the sameCallbackPage-BUA74cVe.js:1 Callback error: Error: Auth callback failed (500)
    at Proxy.o (index-BrDQDEUD.js:6:65554)
    at async CallbackPage-BUA74cVe.js:1:346

### Prompt 60

is it working?
rpc error: code = Internal desc = Multiple exceptions: [Errno 111] Connect call failed ('::1', 5432, 0, 0), [Errno 111] Connect call failed ('127.0.0.1', 5432) this was ossies wotj chat in https://demo.ravencloak.REDACTED

### Prompt 61

neither of it is working, now even file uploads don't show in queued status, it just refreshes and logs out

### Prompt 62

[Request interrupted by user]

### Prompt 63

continue but use chrome extension to test everything yourself

### Prompt 64

<task-notification>
<task-id>b2n39eu9q</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Frontend rebuild from correct cwd, no-cache, with source_type fix" completed (exit code 0)</summary>
</task-notification>

### Prompt 65

still the same issue and also fix the issue where hard reload is causing the spa to logout, use https://supertokens.com/docs/llms-full.txt to learn how to properly use supertokens

### Prompt 66

continue

### Prompt 67

<task-notification>
<task-id>bxpfrlh5k</task-id>
<tool-use-id>toolu_01AJQTLXTXvQ4iFxwAEHGDiu</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Queue PR auto-merge + rebuild + deploy frontend with header-based auth" completed (exit code 0)</summary>
</task-notification>

### Prompt 68

Stil doesn't work, why don't you test using playright or chrome extension

### Prompt 69

let's create a test login to bypass google login for testing purposes

### Prompt 70

This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.

Summary:
1. Primary Request and Intent:
   The user is migrating the Raven demo app from AWS EC2 to a Vultr box (64.176.97.248), then debugging a series of cascading bugs to get a working demo at https://demo.ravencloak.org/raven/. Sequence of explicit intents:
   - Land PR #625 (path-prefix feature) and fix the resulting broken demo
   - Cut re...

### Prompt 71

done

### Prompt 72

<task-notification>
<task-id>bwys780c3</task-id>
<tool-use-id>toolu_01AuYUyGzBb3MQv3YLKQa1YP</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Poll docker.yml run 26325694867 until completion" completed (exit code 0)</summary>
</task-notification>

### Prompt 73

what's happening?

### Prompt 74

done, logged in

### Prompt 75

its merged

### Prompt 76

where is it running the docker command?

### Prompt 77

this failed https://github.REDACTED

### Prompt 78

<task-notification>
<task-id>bsktzf9py</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Wait for PR #672 merge then post-merge docker build" completed (exit code 0)</summary>
</task-notification>

### Prompt 79

Logged in

### Prompt 80

continue

### Prompt 81

check now

### Prompt 82

continue

### Prompt 83

signed in

### Prompt 84

ah okay, then fix our auth? is it a vue js issue?

### Prompt 85

done

### Prompt 86

let's do the follow up tasks as well, i've already decommisioned aws, ignore that one

### Prompt 87

close them

### Prompt 88

did you check with docs upload, url etc if the demo is working using playwright/

### Prompt 89

Run aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25
Run aquasecurity/setup-trivy@3fb12ec12f41e471780db15c232d5dd185dcb514
Run echo "dir=$HOME/.local/bin/trivy-bin" >> $GITHUB_OUTPUT
Run actions/cache/restore@9255dc7a253b0ccc959486e2bca901246202afeb
Cache hit for: trivy-binary-v0.70.0-Linux-X64
Received 43751221 of 43751221 (100.0%), 69.8 MBs/sec
Cache Size: ~42 MB (43751221 B)
/usr/bin/tar -xf /home/runner/work/_temp/79ad6235-f151-4fbf-97fd-172f157a97fd/cache.tzst -P -C /home/...

### Prompt 90

what's next? did you test if document upload and processing is working?

### Prompt 91

check if the docs got processed

### Prompt 92

yes let's use s3 sdk, but where are we storing the documents in the backend. I don't remember setting up any s3 bucket

### Prompt 93

b

### Prompt 94

uploaded

### Prompt 95

refreshing the page after upload, removes the document all together and then upload breaks all together

### Prompt 96

again it dissaperead, test yourself with playwright

### Prompt 97

logged in, you can just click sign in and logs in without going through google, don't ask me the next time

### Prompt 98

rpc error: code = Internal desc = No active 'anthropic' provider config found for org 'b9092473-da0d-4734-ad0f-5f448c694cdc'

### Prompt 99

There should have been a UI flow where the user is given an enpoint to fetch anthropic key and dump in our system for their org

### Prompt 100

that url redirects me to https://demo.ravencloak.org/raven/onboarding

### Prompt 101

Provider Type

Ollama
Display Name
Ollama
Model

Llama 3
Base URL
https://api.example.com/v1
API Key
Ollama runs locally — no API key required, but the field can't be empty. Use any placeholder (e.g. "ollama") and set the Base URL to your daemon.

••••••
This will be set as your default provider since it's the first one.

Cancel
 are the models fetched from local ollama installation, what will be the url. put in the default one and remove the api key field if not required

### Prompt 102

can we not suggest some local tunnel that opens local ollama to raven

### Prompt 103

we should also show urls to install cloudflare or ngrok

### Prompt 104

yes

### Prompt 105

which is the Which is the latest LLM model that I can use which runs locally and performs best according to benchmarks?

### Prompt 106

my m1 macbook pro and for using ollama as the provider for raven

### Prompt 107

Apple M1 Pro

### Prompt 108

Access to catch-mls-smilies-mrs.trycloudflare.com was denied
You don't have authorisation to view this page.
HTTP ERROR 403  --- CONNECTIVITY PRE-CHECKS ----------------------------------------------------
COMPONENT         TARGET                     STATUS  DETAILS
DNS Resolution    region1.v2.argotunnel.com  PASS    DNS Resolved successfully
DNS Resolution    region2.v2.argotunnel.com  PASS    DNS Resolved successfully
UDP Connectivity  region1.v2.argotunnel.com  PASS    QUIC connection succes...

### Prompt 109

Access to vocabulary-supervision-finder-upc.trycloudflare.com was denied
You don't have authorisation to view this page.
HTTP ERROR 403

### Prompt 110

it just gives empty response

### Prompt 111

curl -i https://vocabulary-supervision-finder-upc.trycloudflare.com/api/tags
HTTP/2 403
date: Wed, 27 May 2026 06:14:59 GMT
content-length: 0
cf-ray: a022e1e61dc69f61-MAA
cf-cache-status: DYNAMIC
server: cloudflare

➜  raven git:(fix/llm-provider-add-ollama-enum) ✗

### Prompt 112

raven git:(fix/llm-provider-add-ollama-enum) ✗ cloudflared tunnel --url http://127.0.0.1:11434
2026-05-27T06:11:31Z INF Thank you for trying Cloudflare Tunnel. Doing so, without a Cloudflare account, is a quick way to experiment and try it out. However, be aware that these account-less Tunnels have no uptime guarantee, are subject to the Cloudflare Online Services Terms of Use (https://www.cloudflare.com/website-terms/), and Cloudflare reserves the right to investigate your use of Tunnels for ...

### Prompt 113

works add that comment in the command dialog

### Prompt 114

✕ Connection failed
Test connection
Request failed (404) but the cf endpoint is working https://sustained-copyrights-how-shoppers.trycloudflare.com/

### Prompt 115

even with ollama i'm getting error for chat rpc error: code = Internal desc = No active 'anthropic' provider config found for org 'b9092473-da0d-4734-ad0f-5f448c694cdc'

### Prompt 116

rpc error: code = Internal desc = ollama embed failed: status=400 body='{"error":"model is required"}'

### Prompt 117

rpc error: code = Internal desc = cannot perform operation: another operation is in progress

### Prompt 118

not working for me, not even upload is working. Fix all of them 

File Name    Type    Uploaded    Status
EduCapture-Architecture.pdf    application/pdf    27/05/2026    failed Processing failed
Epoqgird.md    text/markdown    26/05/2026    ready
JobinLawranceResume .md    text/markdown    26/05/2026    ready
Chat with this KB
Ask questions about the documents in this knowledge base.

summarize
No relevant information found.

### Prompt 119

Is the demo up and running? check using chrome extension or playwright, document upload , processing and chat should work fine

### Prompt 120

https://honest-realty-cheats-chances.trycloudflare.com

### Prompt 121

Chat with this KB
Ask questions about the documents in this knowledge base.

is it working?
Unfortunately, I couldn't find any relevant information in your knowledge base to answer this question directly. However, based on general knowledge, without more context or specifics about what "it" refers to (e.g., a system, device, software, etc.), it's difficult for me to provide a definitive answer. Could you please provide more details so I can try and assist you better? No documents were referenced...

### Prompt 122

upload is also not working

### Prompt 123

I'd be happy to summarize for you. However, I don't have any specific information or context to draw upon as there doesn't appear to be a query that matched your knowledge base. I can try to provide a general summary based on common understanding of the term "summarize", but it may not be tailored to your specific needs. If you'd like to rephrase your question or provide more context, I'd be happy to help in any way I can! **No documents were referenced in this response.**

### Prompt 124

Base directory for this skill: /Users/jobinlawrance/.claude/skills/handoff

Write a handoff document summarising the current conversation so a fresh agent can continue the work. Save to the temporary directory of the user's OS - not the current workspace.

Include a "suggested skills" section in the document, which suggests skills that the agent should invoke.

Do not duplicate content already captured in other artifacts (PRDs, plans, ADRs, issues, commits, diffs). Reference them by path or URL ...

### Prompt 125

This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.

Summary:
1. Primary Request and Intent:
   The user has been iteratively debugging and fixing the Raven demo deployment (https://demo.ravencloak.org/raven/), driving toward a working end-to-end flow: Google sign-in → document upload → processing → semantic RAG chat against local Ollama via Cloudflare tunnel. Key explicit requests through t...

### Prompt 126

still getting this error I'm happy to provide a summary, but I must note that I couldn't find any relevant information in your configured knowledge base. However, I can try to provide a general answer based on my knowledge. To summarize something typically means to condense it into a shorter form while still capturing the main points and essential details. It's often used to simplify complex information, make it easier to understand, or convey the essence of a larger concept. If you'd like more ...

### Prompt 127

Request failed: 502

### Prompt 128

Deploy it and clear all caches

### Prompt 129

give url

### Prompt 130

rpc error: code = Internal desc = No active 'anthropic' provider config found for org 'b9092473-da0d-4734-ad0f-5f448c694cdc'

### Prompt 131

<task-notification>
<task-id>b3bdboifl</task-id>
<tool-use-id>toolu_01JTAixGgzoHw6Lo3Etttccy</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>failed</status>
<summary>Background command "Wait for go-api build to finish" failed with exit code 255</summary>
</task-notification>

### Prompt 132

continue

### Prompt 133

is it working now?

### Prompt 134

is it working now?

### Prompt 135

what's the status?

### Prompt 136

https://gay-oriental-bureau-tab.trycloudflare.com

### Prompt 137

<task-notification>
<task-id>bfh5bq0n1</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>failed</status>
<summary>Background command "Wait for build + hot-swap" failed with exit code 255</summary>
</task-notification>

### Prompt 138

<task-notification>
<task-id>bkklbntc6</task-id>
<tool-use-id>toolu_01YXFHH6GrXCA2isX8oHSCE2</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>failed</status>
<summary>Background command "Wait + hot-swap" failed with exit code 255</summary>
</task-notification>

### Prompt 139

Also add a feature in the applciaiton, which runs a frontend cron that tests the connection with the default llm provider, then if connection fails it shows a permanent warning message like a snackbar or a toast at the top right corner with a button that takes you to the LLM provider page which will also show, Connection error

### Prompt 140

it's merged

### Prompt 141

check

### Prompt 142

<task-notification>
<task-id>br21sb9cn</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>failed</status>
<summary>Background command "Wait + hot-swap + verify route" failed with exit code 255</summary>
</task-notification>

### Prompt 143

<task-notification>
<task-id>bbf7xpj39</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Wait + recreate frontend" completed (exit code 0)</summary>
</task-notification>

### Prompt 144

do in parallel

### Prompt 145

what's the status?

### Prompt 146

https://colorado-leisure-man-beyond.trycloudflare.com/
Test connection
Test now
✕ Internal Server Error

### Prompt 147

still getting the same error

### Prompt 148

continue

### Prompt 149

<task-notification>
<task-id>b1tit1vfd</task-id>
<tool-use-id>toolu_01EcqAQ7nuoAEPQYF2N3D9La</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Wait + recreate + verify bundle (with API URL check)" completed (exit code 0)</summary>
</task-notification>

### Prompt 150

continue

### Prompt 151

continue

### Prompt 152

finish the rest of it

### Prompt 153

for url source crawling lets use https://github.com/firecrawl/firecrawl

### Prompt 154

close https://github.com/ravencloak-org/Raven/pull/832 comments

### Prompt 155

build actions are failing, fix those

### Prompt 156

actions failing https://github.com/ravencloak-org/Raven/pull/833

### Prompt 157

https://github.com/ravencloak-org/Raven/pull/832 this too

### Prompt 158

Fix thse in parallel https://github.com/ravencloak-org/Raven/pulls?q=is%3Apr+is%3Aopen+feat+

### Prompt 159

https://github.REDACTED?pr=840 https://github.com/ravencloak-org/Raven/pull/839#issuecomment-4638068254

### Prompt 160

https://github.com/ravencloak-org/Raven/pull/799#issuecomment-4626526121 fix

### Prompt 161

close all the prs

### Prompt 162

dont close fix all of them

### Prompt 163

fix all of them https://github.com/ravencloak-org/Raven/pulls

### Prompt 164

it's all failing

### Prompt 165

Yes, investigate in parallel.

### Prompt 166

check again

