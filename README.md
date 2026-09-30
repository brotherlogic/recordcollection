# recordcollection

[![Coverage Status](https://coveralls.io/repos/github/brotherlogic/recordcollection/badge.svg?branch=master)](https://coveralls.io/github/brotherlogic/recordcollection?branch=master)

`recordcollection` is the central hub for managing a vinyl record collection. It integrates deeply with Discogs and acts as a coordinator for a variety of specialized microservices, providing advanced metadata tracking, automated sale management, and collection organization.

## Features

- **Discogs Synchronization**: Automatically syncs collection and wantlist data with Discogs.
- **Advanced Metadata Tracking**:
    - Physical dimensions (spine width, weight).
    - Condition tracking (media and sleeve).
    - Package score tracking: parsing and validation in range [0, 5] with issue raising on invalid inputs, -1 default for unrated record stubs, and automated ingestion from Discogs custom fields during cacheRecord synchronization.
    - Rip quality tracking: support for rip quality scores in range [0, 100] via `ripped_quality` metadata field, automatic quality retrieval from the `recorder` service on `last_rip_date` mutation in `UpdateRecord`, sentinel (`-1`) clearing of rip date and quality, Prometheus metric tracking via `recordcollection_quality_fetch_result`, and comprehensive unit test coverage for rip date setting, re-ripping score updates, field updates preservation, clearing, and error/invalid score handling.
    - Out of play status tracking: support for the `out_of_play` boolean field with proto3 field presence on `ReleaseMetadata` to track whether records are temporarily or permanently removed from active play or rotation, verified through gRPC server `UpdateRecord` unit tests covering true toggling, explicit false reset preservation via `proto.Merge`, dirty status preservation, field omission idempotency, and non-existent record handling.
    - Custom categories and "purgatory" states (needs labels, needs rip, etc.).
    - **IID Validation & Cleaning**: On every startup, all eight internal cache maps (`InstanceToFolder`, `InstanceToCategory`, `InstanceToUpdate`, `InstanceToUpdateIn`, `InstanceToMaster`, `InstanceToId`, `InstanceToRecache`, `InstanceToLastSalePriceUpdate`) are scrubbed of any negative instance IDs — legacy artifacts of historical int32 overflow. Cleaned data is persisted back to the keystore so the stale entries do not reappear on the next restart.
    - **Validation Dirty Flag Logic**: Corrected the validation dirty flag behavior. Now, `VALIDATE` category records are only marked as dirty and their `LastValidate` timestamp set when explicitly transitioning into the `VALIDATE` category. This prevents subsequent updates (like background syncs) from re-marking them as dirty and resetting `LastValidate`.
    - **Gramophile Updates**: When a record has `NeedsGramUpdate` set, a fanout task is enqueued for one minute in the future in `record_fanout` to pull updated details from Discogs, including updates initiated by Grambridge.

- **Automated Sale Management**:
    - **Quality Gating & Rip-First Routing**: In `UpdateRecord`, scoring a record in `STAGED_TO_SELL` with sell rating (3) evaluates `ripped_quality`. Records with `ripped_quality < 80` (or unset/0) are diverted to `RIP_THEN_SELL` with score reset (`-1`) to block sale until verified. Records with quality >= 80 escalate to `SOLD` to enter the sale pipeline. Scoring records in `RIP_THEN_SELL` unconditionally escalates them to `SOLD`. Unit tests verify low quality rip diversion, missing quality diversion, quality 80 and 95 sale escalation, keeper rating preservation, unconditional RIP_THEN_SELL sale escalation for score and keeper ratings, and precondition failure on missing notes or sleeve condition.
    - **Listing Generation**: Integrates with an external gRPC service to automatically generate rich, descriptive sale listings based on record condition and user notes, utilizing the local Ollama model setting for description generation.
    - **Dynamic Pricing**: Tracks and updates sale prices based on market data.
    - **Blocked Records**: Automatically removes records from sale and updates their properties if they are marked as blocked from sale on Discogs.
    - **Inventory Control**: View and manage current Discogs inventory directly through the service.
- **Microservice Orchestration**: Coordinates with other services in the ecosystem:
    - `recordmover`: Physical relocation of records between folders (supports full 64-bit instance IDs).
    - `recordscores`: Advanced scoring and rating logic.
    - `recordsorganiser`: Folder quota and organization management.
    - `recordfanout`: Broadcasts updates to dependent services.
    - `recorder`: Rip quality score retrieval via gRPC (`QualityService.GetQuality`) configured via `--recorder_address` (defaulting to `recorder:8087`).
- **Audit & History**: Tracks listen times, auditions, and historical updates for every record in the collection.
- **Monitoring**: Built-in Prometheus metrics for tracking collection status, service health, and loop latencies.

## Architecture

`recordcollection` is a Go-based gRPC service. It serves as the primary data store (backed by `keystore`) and API gateway for the record collection ecosystem.

### Key Components

- **gRPC API**: Defined in `proto/recordcollection.proto`, offering services for record querying, updates, and collection management.
- **Discogs Integration**: Uses `godiscogs` for direct communication with the Discogs API.
- **Storage**: Leverages `keystore` for persistent, versioned storage of record metadata.
- **Metrics**: Exports detailed Prometheus metrics, including record states, folder sizes, and fanout status.

## Getting Started

### Prerequisites

- **Go**: Version 1.26.1 or higher.
- **Protobuf**: `protoc` compiler with Go plugins.
- **Discogs API Token**: Required for synchronization features.

### Installation & Build

1. Clone the repository:
   ```bash
   git clone https://github.com/brotherlogic/recordcollection.git
   cd recordcollection
   ```

2. Initialize and build the project using the provided script:
   ```bash
   ./build.sh
   ```
   This script vendors dependencies and compiles the protobuf definitions.

### Running the Service

Start the service by providing your Discogs token:
```bash
go run recordcollection.go --token <your_discogs_token>
```

### CLI
The service includes a CLI for common management tasks:
```bash
go run recordcollection_cli/cli.go <command>
```
Key commands:
- `last_week_listens`: Lists 12-inch records listened to in the last week, ordered by score.
- `listsales`: Lists all records currently listed for sale.
- `bad_sales`: Lists records for sale that are blocked from sale on Discogs.
- `pull_blocked`: Lists sold 12-inch records that were physically removed from active sale due to being blocked.
- `adjust`: Enqueues records for fanout if they are not already in the queue and not marked as SOLD_ARCHIVE.
- `out_of_play <instance_id> <true|false>`: Sets or clears the `out_of_play` status of a record in collection metadata.
- `sleeve_boxsets`: Prints all sleeve boxsets (`ReleaseMetadata_BOX_SET`), outputting the category and instance ID for each record.

## Development

### Resource Management
The background processing loops and gRPC integrations have been fully optimized to ensure strict context cancellation and gRPC client connection hygiene, preventing resource and memory leaks during high-frequency update runs.

### TDD Workflow
We follow a strict Test-Driven Development (TDD) process. Always write a failing test before adding a new feature or fixing a bug.
- Run tests: `go test ./...`
- Mock external services to support unit tests and avoid side effects.

### Feature Completion

Refer to [ISSUES.md](file:///workspaces/recordcollection/ISSUES.md) for details on the issue processing workflow lifecycle and label transitions.
Once you've finished a change or feature, run the `finish.md` workflow.


### Protobuf Updates
If you modify the API, update `proto/recordcollection.proto` and run `./build.sh` to regenerate the Go bindings.

## Monitoring
Metrics are available via Prometheus. Key metrics include:
- `recordcollection_recordstate`: Number of records in each category.
- `recordcollection_recordfolder`: Size of each physical folder.
- `recordcollection_loop_latency`: Performance tracking for internal processing loops.
- `recordcollection_sale_descriptor_result`: Counter tracking the gRPC response status codes for calls made to the outbound sale descriptor service.