#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ARCH=${GOARCH:-arm64}
OUTPUT=${OUTPUT:-"$ROOT_DIR/load-gen-linux-$ARCH"}

case "$ARCH" in
    arm|arm64)
        ;;
    *)
        printf 'unsupported GOARCH: %s (expected arm or arm64)\n' "$ARCH" >&2
        exit 1
        ;;
esac

cd "$ROOT_DIR"
GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 \
    go build -trimpath -ldflags '-s -w' -o "$OUTPUT" .

printf 'built linux/%s: %s\n' "$ARCH" "$OUTPUT"