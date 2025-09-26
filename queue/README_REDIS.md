# Redis Queue Implementation

This document describes the Redis-based queue implementation that provides persistent, scalable message queuing using Redis as the backend storage.

## Overview

The Redis queue implementation (`Redis`) provides the same interface as the memory-based queue (`QueueBase`) but uses Redis for persistent storage. This allows messages to survive application restarts and enables distributed message processing across multiple application instances.

## Features

- **Persistent Storage**: Messages are stored in Redis and survive application restarts
- **Distributed Processing**: Multiple application instances can share the same queue
- **Atomic Operations**: Message claiming and processing use Redis atomic operations
- **Thread-Safe**: All operations are thread-safe through Redis transactions
- **Scalable**: Can handle high message throughput
- **Compatible**: Implements the same `Queuer` interface as the memory queue

## Redis Data Structure

The Redis queue uses the following Redis data structures:

- `queue:messages` (Hash): Stores serialized message data, keyed by message ID
- `queue:unclaimed` (Set): Contains message IDs that are available for processing
- `queue:claimed` (Set): Contains message IDs that have been claimed for processing
- `queue:processed` (Set): Contains message IDs that have been processed
- `queue:keys:{key}` (Set): Contains message IDs grouped by message key

### LER TIPO SORTED SET gqueue:queue:messages:eventName.test_event

**Buscando o total de itens**

```sh
ZCARD gqueue:queue:messages:eventName.test_event
```

**Buscando os itens**

```sh
ZRANGE gqueue:queue:messages:eventName.test_event 0 -1 WITHSCORES
```
