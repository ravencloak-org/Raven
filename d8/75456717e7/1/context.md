# Session Context

## User Prompts

### Prompt 1

what's the status?

### Prompt 2

what's the status?

### Prompt 3

Let's work on the other items from the milestone.

### Prompt 4

Let's start with the important ones. WhatsApp is not that important. I mean, web art is sure edge optimization, maybe later enterprise connector. I think maybe we can start that. Let me know what are the parallel ones that we can pick up.

### Prompt 5

We run the whole stack locally to test it out before I deploy it.

### Prompt 6

let's use a different server to host everything, it's a fresh server. let's deploy the cms, the demo and the dark variant all here. ssh ubuntu@ec2-65-0-85-115.ap-south-1.compute.amazonaws.com also along with that let's create a docker compose page for this specifically.

### Prompt 7

I'm pretty sure it's very bare, so if you can create an infra as code which will install everything from top to bottom except our current stack, that will be really great. I'm sure there will be an online tool that will install Docker, PM2, and Node.js, and I'll tell you what other stack I need. Till then, you figure out the Ansible, and I'll give you the list of all the software that I want you to install, okay. ?

### Prompt 8

SSH keypath for what exactly? What is this inventory? Also ignore the CMS part; that's from a different project. I'll go there and do it. Ignore it for now. We'll just take care of the current repository, which is Raven.

### Prompt 9

Right, the key that I use for SSH is the id_ed25519. That's the key used. I don't think you need to specify it if you just SSH into the server; it'll automatically pick it up. For now, these are the only main stack I want. Does bescel take care of open telemetry as well, or is it only for metrics?

### Prompt 10

So, let's go for that.

### Prompt 11

What is the vault password?

### Prompt 12

Can we remove this vault thingy that is required?

### Prompt 13

yes

### Prompt 14

TASK [docker : Add Docker repository] *********************************************************************************************************************************************************
[DEPRECATION WARNING]: INJECT_FACTS_AS_VARS default to `True` is deprecated, top-level facts will not be auto injected after the change. This feature will be removed from ansible-core version 2.24.
Origin: /Users/jobinlawrance/Project/raven/deploy/ansible/roles/docker/tasks/main.yml:19:15

17     - name: Ad...

### Prompt 15

TASK [raven-stack : Start Raven stack] ********************************************************************************************************************************************************
[ERROR]: Task failed: Module failed: non-zero return code
Origin: /Users/jobinlawrance/Project/raven/deploy/ansible/roles/raven-stack/tasks/main.yml:32:3

30   changed_when: true
31
32 - name: Start Raven stack
     ^ column 3

fatal: [raven-server]: FAILED! => {"changed": true, "cmd": ["docker", "compose",...

### Prompt 16

TASK [admin-tools : Copy Glance config] *******************************************************************************************************************************************************
changed: [raven-server]

TASK [admin-tools : Pull admin tool images] ***************************************************************************************************************************************************
[ERROR]: Task failed: Module failed: non-zero return code
Origin: /Users/jobinlawrance/Proj...

### Prompt 17

is everything up there?

### Prompt 18

is everything up there?

### Prompt 19

Well, that's mostly the software stack. Let's go ahead and install Raven.

