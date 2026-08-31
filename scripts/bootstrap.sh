#!/usr/bin/env bash
#
# Instala las herramientas de desarrollo en las versiones declaradas en
# .tool-versions. Es idempotente: si una herramienta ya está en la versión
# correcta, no hace nada.
#
# Uso: ./scripts/bootstrap.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOL_VERSIONS="${REPO_ROOT}/.tool-versions"

GO_INSTALL_DIR="/usr/local/go"
GOBIN="${HOME}/go/bin"

info()  { printf '\033[0;34m::\033[0m %s\n' "$1"; }
ok()    { printf '\033[0;32m ✓\033[0m %s\n' "$1"; }
warn()  { printf '\033[0;33m ！\033[0m %s\n' "$1" >&2; }
fail()  { printf '\033[0;31m ✗\033[0m %s\n' "$1" >&2; exit 1; }

# Lee la versión de una herramienta desde .tool-versions.
tool_version() {
  local tool="$1" version
  version="$(awk -v t="$tool" '$1 == t { print $2; exit }' "$TOOL_VERSIONS")"
  [ -n "$version" ] || fail "no hay versión declarada para '$tool' en .tool-versions"
  printf '%s' "$version"
}

check_prerequisites() {
  [ -f "$TOOL_VERSIONS" ] || fail "no se encontró $TOOL_VERSIONS"

  local missing=()
  for cmd in curl tar awk sudo; do
    command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
  done

  if [ ${#missing[@]} -gt 0 ]; then
    fail "faltan comandos requeridos: ${missing[*]}"
  fi

  case "$(uname -s)" in
    Linux) ;;
    *) fail "este script solo soporta Linux; detectado: $(uname -s)" ;;
  esac
}

# Go instalado desde el gestor de paquetes de la distribución queda fuera del
# control del proyecto: su versión y su calendario de actualización no son
# decisiones nuestras. Ver ADR-0002.
warn_if_distro_go() {
  if command -v dpkg >/dev/null 2>&1 && dpkg -S "$(command -v go)" >/dev/null 2>&1; then
    warn "hay un Go instalado desde el gestor de paquetes del sistema."
    warn "eliminarlo con: sudo apt remove golang-go"
  fi
}

install_go() {
  local want current arch
  want="$(tool_version golang)"

  if command -v go >/dev/null 2>&1; then
    current="$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')" || current=""
    if [ "$current" = "$want" ]; then
      ok "go $want"
      return
    fi
    info "go $current instalado, se requiere $want"
    warn_if_distro_go
  else
    info "go no encontrado, instalando $want"
  fi

  case "$(uname -m)" in
    x86_64)  arch="amd64" ;;
    aarch64) arch="arm64" ;;
    *) fail "arquitectura no soportada: $(uname -m)" ;;
  esac

  local tarball="go${want}.linux-${arch}.tar.gz"
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN

  info "descargando $tarball"
  curl -fsSL "https://go.dev/dl/${tarball}" -o "${tmp}/${tarball}" \
    || fail "no se pudo descargar $tarball"

  sudo rm -rf "$GO_INSTALL_DIR"
  sudo tar -C "$(dirname "$GO_INSTALL_DIR")" -xzf "${tmp}/${tarball}"

  export PATH="${GO_INSTALL_DIR}/bin:${GOBIN}:${PATH}"
  hash -r
  ok "go $want instalado"
}

install_golangci_lint() {
  local want current
  want="$(tool_version golangci-lint)"

  export PATH="${GO_INSTALL_DIR}/bin:${GOBIN}:${PATH}"

  if command -v golangci-lint >/dev/null 2>&1; then
    current="$(golangci-lint --version 2>/dev/null | awk '{print $4}')" || current=""
    if [ "$current" = "$want" ]; then
      ok "golangci-lint $want"
      return
    fi
    info "golangci-lint $current instalado, se requiere $want"
  else
    info "golangci-lint no encontrado, instalando $want"
  fi

  go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${want}" \
    || fail "no se pudo instalar golangci-lint v${want}"
  ok "golangci-lint $want instalado"
}

configure_path() {
  local line='export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH'
  local shell_rc="${HOME}/.bashrc"

  [ -n "${ZSH_VERSION:-}" ] && shell_rc="${HOME}/.zshrc"

  if [ -f "$shell_rc" ] && grep -qF 'usr/local/go/bin' "$shell_rc"; then
    ok "PATH ya configurado en $(basename "$shell_rc")"
    return
  fi

  printf '\n%s\n' "$line" >> "$shell_rc"
  ok "PATH añadido a $(basename "$shell_rc")"
  warn "ejecute 'source $shell_rc' o abra una terminal nueva"
}

verify() {
  export PATH="${GO_INSTALL_DIR}/bin:${GOBIN}:${PATH}"
  hash -r

  local failed=0 want current

  want="$(tool_version golang)"
  current="$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')" || current="ausente"
  if [ "$current" != "$want" ]; then
    warn "go: se esperaba $want, se encontró $current"
    failed=1
  fi

  want="$(tool_version golangci-lint)"
  current="$(golangci-lint --version 2>/dev/null | awk '{print $4}')" || current="ausente"
  if [ "$current" != "$want" ]; then
    warn "golangci-lint: se esperaba $want, se encontró $current"
    failed=1
  fi

  [ "$failed" -eq 0 ] || fail "la verificación no pasó"
  ok "todas las herramientas en la versión declarada"
}

main() {
  info "instalando herramientas desde .tool-versions"
  check_prerequisites
  install_go
  install_golangci_lint
  configure_path
  verify
  info "listo. verifique el proyecto con: make test && make lint"
}

main "$@"
