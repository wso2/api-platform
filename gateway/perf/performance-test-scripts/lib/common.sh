#!/bin/bash
# Shared helpers for performance-test-scripts.
# Usage: source "${MANUAL_DIR}/lib/common.sh"   (after env.example)

# Re-exec with bash 4+ (macOS default bash is 3.2).
require_bash4() {
    if ((BASH_VERSINFO[0] < 4)); then
        for _bash in /opt/homebrew/bin/bash /usr/local/bin/bash /usr/bin/bash; do
            if [[ -x "$_bash" ]] && "$_bash" -c '((BASH_VERSINFO[0] >= 4))' 2>/dev/null; then
                exec "$_bash" "$0" "$@"
            fi
        done
        echo "Error: bash 4+ required (install: brew install bash)" >&2
        exit 1
    fi
}

# True on Ubuntu EC2 (apt available).
is_ubuntu() {
    [[ "$(uname -s)" == "Linux" ]] && command -v apt-get >/dev/null 2>&1
}

# True on Amazon Linux EC2 (ec2-user, yum/dnf).
is_amazon_linux() {
    [[ -f /etc/os-release ]] && grep -qiE 'amazon linux' /etc/os-release
}

linux_pkg_manager() {
    if is_ubuntu; then
        echo apt
    elif command -v dnf >/dev/null 2>&1; then
        echo dnf
    elif command -v yum >/dev/null 2>&1; then
        echo yum
    else
        echo ""
    fi
}

# Install packages on Ubuntu or Amazon Linux EC2; no-op on macOS.
install_linux_packages() {
    local pm
    pm="$(linux_pkg_manager)"
    [[ -n "$pm" ]] || return 0
    if [[ "$pm" == apt ]]; then
        sudo apt-get update
        sudo apt-get install -y "$@"
    else
        sudo "$pm" install -y "$@"
    fi
}

# Backward-compatible alias.
install_ubuntu_packages() {
    install_linux_packages "$@"
}

# Java 17 + Maven for backend/jmeter setup on EC2.
install_java17_and_maven() {
    if is_ubuntu; then
        install_linux_packages openjdk-17-jdk maven curl netcat-openbsd
    elif is_amazon_linux; then
        # curl-minimal is preinstalled on AL2023; installing curl often conflicts.
        install_linux_packages java-17-amazon-corretto-devel maven
        install_linux_packages nmap-ncat 2>/dev/null || true
    else
        echo "Install Java 17 and Maven manually, then re-run setup." >&2
        return 1
    fi
    command -v java >/dev/null || { echo "java not on PATH after install" >&2; return 1; }
    java -version
}

# Path to shaded Netty mock JAR after Maven build.
resolve_netty_jar() {
    find "${PERF_ROOT}/performance-common/components/netty-http-echo-service/target" \
        -maxdepth 1 -name 'netty-http-echo-service*.jar' ! -name 'original-*' 2>/dev/null | head -1
}

# Run docker compose (plugin) or legacy docker-compose.
docker_compose() {
    if docker compose version >/dev/null 2>&1; then
        docker compose "$@"
    elif command -v docker-compose >/dev/null 2>&1; then
        docker-compose "$@"
    else
        echo "Docker Compose not found. On Amazon Linux: sudo dnf install -y docker-compose-plugin" >&2
        exit 1
    fi
}

# Install docker compose v2 plugin (AL2023 has no docker-compose-plugin RPM).
install_docker_compose_plugin() {
    if docker compose version >/dev/null 2>&1; then
        return 0
    fi

    local arch plugin_dir plugin_path url
    arch="$(uname -m)"
    for plugin_dir in /usr/libexec/docker/cli-plugins /usr/local/lib/docker/cli-plugins; do
        sudo mkdir -p "$plugin_dir"
        plugin_path="${plugin_dir}/docker-compose"
        url="https://github.com/docker/compose/releases/latest/download/docker-compose-linux-${arch}"
        echo "Installing docker compose plugin -> ${plugin_path}" >&2
        if sudo curl -fsSL "$url" -o "$plugin_path"; then
            sudo chmod +x "$plugin_path"
            if docker compose version >/dev/null 2>&1; then
                docker compose version
                return 0
            fi
        fi
    done

    # Fallback: standalone docker-compose binary
    if ! command -v docker-compose >/dev/null 2>&1; then
        sudo curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-${arch}" \
            -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
    fi
    docker-compose version >/dev/null 2>&1 || docker compose version >/dev/null 2>&1 || {
        echo "Failed to install docker compose" >&2
        return 1
    }
}

# Docker + Compose on EC2 gateway host.
install_docker_ec2() {
    if is_amazon_linux; then
        install_linux_packages docker jq
        sudo systemctl enable --now docker
        install_docker_compose_plugin
    else
        install_linux_packages docker.io docker-compose-plugin jq curl
        install_docker_compose_plugin 2>/dev/null || true
    fi
    sudo usermod -aG docker "${USER:-$(whoami)}" 2>/dev/null || true
}

# Build Netty mock JAR if missing.
ensure_netty_jar() {
    local jar
    jar="$(resolve_netty_jar)"
    if [[ -n "$jar" && -f "$jar" ]]; then
        echo "$jar"
        return 0
    fi
    echo "Building Netty mock JAR..." >&2
    (cd "${PERF_ROOT}/performance-common" && mvn -q package -DskipTests -Dfindbugs.skip=true \
        -pl components/netty-http-echo-service -am)
    resolve_netty_jar
}
