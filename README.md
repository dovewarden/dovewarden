# Project Lightfeather

`lightfeather` implements an external replication management daemon for Dovecot servers, resembling the feature removed in Dovecot 2.4. It receives event notifications when mailboxes are modified through the HTTP event API, stores these in a priority queue in Redis, and uses the doveadm HTTP API to trigger replications to the remote Dovecot server.

## Features

- **Event-Driven Replication**: Listens for mailbox modification events from Dovecot and triggers replication accordingly.
- **Supported Protocols**:
  - **IMAP**, relying on the same notification events Dovecot 2.3 was using.
  - ~~**Sieve**~~ can be added in the future
  - ~~**POP3**~~ can be added in the future
- **Persistent priority queue**: Utilizes Redis to maintain a priority queue for replication tasks.
  - allows persistent storage of replication tasks also across daemon restarts
  - allows highly available (and scaling out, if that is ever required) setups with multiple `lightfeather` instances sharing the same Redis backend, not strictly requiring complete reconciliation on Dovecot startup
  - retry mechanism for failed replications
  - background reconciliation of all users to ensure consistency also without events

## Architecture

The architecture of `lightfeather` consists of the following components:

- **Dovecot Server**: The primary mail server where mailboxes are hosted. It is configured to send HTTP event notifications to the `lightfeather` daemon whenever a mailbox is modified.
- **lightfeather daemon**: The core component written in Golang listens for HTTP event notifications from the Dovecot server. It processes these notifications and stores them in a priority queue in Redis.
- **Redis**: A fast, in-memory data structure store used as a priority queue to manage replication tasks. `lightfeather` uses Redis to store and retrieve replication tasks efficiently.
