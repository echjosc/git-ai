bin := "gai"
prefix := env_var_or_default("PREFIX", home_directory() / ".local/bin")

# List available recipes (default when you just run `just`).
default:
    @just --list

# Build the binary into the current directory.
build:
    go build -o {{bin}} .

# Build, then move into place on your PATH.
install: build
    mkdir -p {{prefix}}
    mv {{bin}} {{prefix}}/{{bin}}
    @echo "installed {{bin}} -> {{prefix}}/{{bin}}"

# Run the test suite.
test:
    go test ./...

# Vet + fail if anything is unformatted.
check:
    go vet ./...
    @test -z "$(gofmt -l .)" || (echo "unformatted files:"; gofmt -l .; exit 1)

# Format in place.
fmt:
    gofmt -w .

# Remove the built binary.
clean:
    rm -f {{bin}}

# Build and run, passing through any args: `just run commit -ay`
run *args: build
    ./{{bin}} {{args}}
