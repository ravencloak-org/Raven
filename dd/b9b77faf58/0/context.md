# Session Context

## User Prompts

### Prompt 1

Let's brainstorm and check if we can use a low-level Golang feature called as EBPF: I am attaching a link to an article that explains it. Let me know if that can be of any use to our current Raven project and, if so, how we will benefit from it. https://sazak.io/articles/an-applied-introduction-to-ebpf-with-go-2024-06-06

### Prompt 2

Base directory for this skill: /Users/jobinlawrance/.claude/plugins/cache/claude-plugins-official/superpowers/5.0.5/skills/brainstorming

# Brainstorming Ideas Into Designs

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementatio...

### Prompt 3

Let's add the strongest fit and the strong fit, which is the eBPF-based
  free filtering and kernel-level observability, as well as the security (or
  drill 1, 2, and 3, all of them) in the good to have milestone or
  something of that sort. Optimization, maybe, or not. Just note everything
  down and keep it in the milestone so that we can pick it up later.

### Prompt 4

What is the utility difference between Open Observe and Cygnos? I know Open Observe does not use an OLAP database like ClickHouse, but is that actually beneficial when the files are residing in Open Observe and when we are collecting a lot of data? Will Cygnos, or any other wrapper stack on top of resilient OLAP like ClickHouse, make sense? Even if I don't go with Cygnos, there is a quick bit with Jaeger on top of it, which I am assuming still uses ClickHouse only. Will that be a better option? ...

### Prompt 5

I wasn't thinking of Jaeger with ClickHouse as the community plugin, but there is some other software called QuickBit, which uses ClickHouse and also supports Jaeger. Just look into it. Running it on a Raspberry Pi is not the priority anyway. The server will be run on my cloud for the enterprise users. It's only for the experimental, self-hostable people. The Raspberry Pi will be an option, and even then they can choose not to install the observability stack or swap a particular observability st...

### Prompt 6

Also, do you think, as data grows infinitely large, instead of Postgres DB, should we, at an enterprise level, think about adding ClickHouse and its vector capabilities? Which one would be faster, efficient, and something that can handle a lot of data? I'm thinking in that sense, because now that we can ingest any sort of data from our ingestion pipeline using parsers, OCRs, and crawlers, this can be used by educational institutions as well to ingest:
- a whole research paper
- a whole book
- a ...

### Prompt 7

Also, as an enterprise pro version, we can sell a take-it-to-your-own-premise enterprise version where In R Stack will run on the data that any particular company already has, instead of them having to manually upload their files, their database dumps, or whatever. Why don't we create something called connectors and allow these connectors to connect to a database, to a file storage system, to parakeet files, and you name it, whatever an enterprise software company will have? Based on that, the u...

### Prompt 8

Also, about the open telemetry option that I was saying about Jaeger with ClickHouse, it's called Quickwit, and I'm sharing a link for that as well. https://quickwit.io/blog/quickwit-0.5

### Prompt 9

And yes, let's brainstorm first. Everything we'll note down are the recommended things that we can pick up and then move forward after that.

### Prompt 10

Now let's first finish brainstorming. I understood that we can't use Quickwit. Let's go with OpenObserve for now, and maybe SigNoz; we'll reevaluate which UI I like better later. Again, the ClickHouse with Qubit Vector URL that I shared with you, will that make sense for vector embeddings or vector use purposes of ours when we grow to that scale and data level, as I mentioned earlier?

### Prompt 11

You can continue.

### Prompt 12

Actually, I would like to go with option C.

### Prompt 13

What is the trade-off if I'm using Postgres connector via Airbyte Postgres or ClickHouse or whatever?

### Prompt 14

Let's not go with native connectors to begin with. Let's just rely on Airbyte. I understand that the CDC will be handled by the right after log, but what about the existing data? How will that be pulled from Airbyte? Will that have a trade-off? By trade-off, I don't mean the compute or the CPU usage or the memory usage that will anyway be running in the clients or my powerful servers, right? That is not an issue, but is there any other speed trade-off or efficiency trade-off or data corruption i...

### Prompt 15

Yeah, then it makes sense. For the later future roadmaps, when we go about doing Raven Pro, which means using Raven on clients' premises, or when we open up Airbyte and the plethora of connectors, all of these won't be one of the MVP, obviously. Let's move it to one of the roadmaps and then go to the next item that I had addressed.

### Prompt 16

Yeah, so you didn't tell me: does ClickHouse plus Qubit make sense for a larger vector embedding as a knowledge database? I am not talking about ClickHouse plus Qubit vector path for observability. I am not sure what that entails, but I was talking about a replacement for Postgres and ParadeDB, perhaps. Whenever the data scale becomes that huge, I am sure it won't be happening anytime soon, but I just like to explore the possibility.

### Prompt 17

Yeah, I'm not sure. Is there any industry standard way of labeling data? While I was pondering data warehouses and data lakes having a labeling feature built in, I'm not sure if it actually does. Just check if data lakes and data warehouses like Snowflake, etc., have their own internal labeling. I'm sure companies label their data, I mean, storing it in different tables in itself is labeling, but I am talking about something a little more on top of that. What if a single table can contain data f...

### Prompt 18

I don't want to build a labeling system on my own. We will just rely on something that is already there. If, by any chance, there are no labels that exist in a database like Postgres or something, then we can just give us a very simple labeling feature, or not even labeling. Basically, they will come up with the sections that they want to expose to their voice agent and their chatbot. Let's say it's a particular program in a college, so only MBA data needs to be enabled for voice and stuff, so t...

### Prompt 19

I think Both would be great. I mean, if they are not tech-savvy, then I don't think they will go for YAML and stuff. If it is a self-hosting situation, then yeah, if they can self-host, I'm sure they can also configure it. You're correct that we will keep the labeling UI as part of the cloud-providing feature, and it will be paid. That is the next topic that I want to discuss with you: how are we going to separate some particular features only for paid enterprise users or pro users? How do we pr...

### Prompt 20

Ah yeah, that sounds about great also, since we can't charge per seat for the strangers and since we will be deploying Google Analytics or maybe post hoc, which will actually capture the end user as well, so we can charge based on the user information that they need.
- Users coming in being converted automatically will be one aspect of it; that's fine. The company gets the money if the user actually goes ahead and buys it, but let's say if the company wants to further pursue them and needs the u...

### Prompt 21

Also, self-hosting, I think, would make sense for a startup or a place where their revenue is lower than a certain amount. If an enterprise company wants to self-host, then I think after a point it should be chargeable, because they will be eligible for upstream fixes and new features in the future as well. Why should it be free? Let's say we have a very smart SaaS company. I don't want them to just self-host and rip off my product entirely without having to pay me.

### Prompt 22

Yes, let's go ahead and do that. What tier boundaries do you want me to adjust? Can you explain?

Also, I had a question about how we restrict these features if it is being self-forced. I am guessing the binary that is generated will have all of these features baked into it, right? If it is baked into it, then how do we stop it? Feature flag? I mean, any smart company can just clone my repo and remove those flags and run it for free. How does this work?

### Prompt 23

Then let's also set up the directories responsible for enterprise ones with different enterprise license and the other ones with the Apache or the MIT license that you suggested. Also update our milestones, the existing ones as well, because some of the features we are building right now might be a Pro or an Enterprise one. If any of them were existing, let's move it to the EE folder. If upcoming milestones and issues have any Pro feature or business feature, then let's develop it in the Enterpr...

### Prompt 24

Create a PR for this. Check if there are any code reviews. Buy a code rabbit. See if all the actions are working fine, and only then, when everything is green, you should squash and release and squash and merge.

### Prompt 25

[Request interrupted by user for tool use]

### Prompt 26

There were changes requested. Please go through them, address them, and resolve it. You know the drill.

### Prompt 27

[Request interrupted by user]

### Prompt 28

<task-notification>
<task-id>bspch9hot</task-id>
<tool-use-id>REDACTED</tool-use-id>
<output-file>REDACTED.output</output-file>
<status>completed</status>
<summary>Background command "Watch PR checks until they complete" completed (exit code 0)</summary>
</task-notification>

### Prompt 29

[Request interrupted by user]

### Prompt 30

There were changes requested. Please go through them, address them, and resolve it. You know the drill.

### Prompt 31

Were all the changes I asked committed to the milestone and issues? Were they updated?

