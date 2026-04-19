# recordcollection

[![Coverage Status](https://coveralls.io/repos/github/brotherlogic/recordcollection/badge.svg?branch=master)](https://coveralls.io/github/brotherlogic/recordcollection?branch=master)

`recordcollection` is the central hub for managing a vinyl record collection. It integrates deeply with Discogs and acts as a coordinator for a variety of specialized microservices, providing advanced metadata tracking, automated sale management, and collection organization.

## Features

- **Discogs Synchronization**: Automatically syncs collection and wantlist data with Discogs.
- **Advanced Metadata Tracking**:
    - Physical dimensions (spine width, weight).
    - Condition tracking (media and sleeve).
    - Custom categories and "purgatory" states (needs labels, needs rip, etc.).
- **Automated Sale Management**:
    - **Listing Generation**: Integrates with an external gRPC service to automatically generate rich, descriptive sale listings based on record condition and user notes.
    - **Dynamic Pricing**: Tracks and updates sale prices based on market data.
    - **Blocked Records**: Automatically removes records from sale and updates their properties if they are marked as blocked from sale on Discogs.
    - **Inventory Control**: View and manage current Discogs inventory directly through the service.
- **Microservice Orchestration**: Coordinates with other services in the ecosystem:
    - `recordmover`: Physical relocation of records between folders.
    - `recordscores`: Advanced scoring and rating logic.
    - `recordsorganiser`: Folder quota and organization management.
    - `recordfanout`: Broadcasts updates to dependent services.
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

## Development

### TDD Workflow
We follow a strict Test-Driven Development (TDD) process. Always write a failing test before adding a new feature or fixing a bug.
- Run tests: `go test ./...`
- Mock external services to support unit tests and avoid side effects.

### Feature Completion
Once a feature is finished, ensure all tests pass and follow the project's standard workflow for branch cleanup and finalization.

### Protobuf Updates
If you modify the API, update `proto/recordcollection.proto` and run `./build.sh` to regenerate the Go bindings.

## Monitoring
Metrics are available via Prometheus. Key metrics include:
- `recordcollection_recordstate`: Number of records in each category.
- `recordcollection_recordfolder`: Size of each physical folder.
- `recordcollection_loop_latency`: Performance tracking for internal processing loops.