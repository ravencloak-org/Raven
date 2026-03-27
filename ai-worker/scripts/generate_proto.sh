#!/usr/bin/env bash
# Generate Python gRPC stubs from the shared proto definitions.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$(dirname "$PROJECT_DIR")/proto"
GEN_DIR="$PROJECT_DIR/raven_worker/generated"

mkdir -p "$GEN_DIR"

echo "Generating Python gRPC stubs from $PROTO_DIR -> $GEN_DIR"

python -m grpc_tools.protoc \
    --proto_path="$PROTO_DIR" \
    --python_out="$GEN_DIR" \
    --grpc_python_out="$GEN_DIR" \
    --pyi_out="$GEN_DIR" \
    "$PROTO_DIR/ai_worker.proto"

# Fix relative imports in generated gRPC stub to use package-relative imports.
# grpc_tools generates `import ai_worker_pb2 as ...` but we need it to be
# `from . import ai_worker_pb2 as ...` since the files live inside a package.
if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' 's/^import ai_worker_pb2 as/from . import ai_worker_pb2 as/' "$GEN_DIR/ai_worker_pb2_grpc.py"
else
    sed -i 's/^import ai_worker_pb2 as/from . import ai_worker_pb2 as/' "$GEN_DIR/ai_worker_pb2_grpc.py"
fi

echo "Done. Generated files:"
ls -la "$GEN_DIR"/ai_worker_*
