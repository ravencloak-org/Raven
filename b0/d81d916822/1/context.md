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

