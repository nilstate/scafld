#!/usr/bin/env sh
# scafld installer
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
#
# Or after cloning:
#   git clone https://github.com/nilstate/scafld.git ~/.scafld
#   ~/.scafld/install.sh
set -e

SCAFLD_HOME="${SCAFLD_HOME:-$HOME/.scafld}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

echo "Installing scafld..."

# Clone or update
if [ -d "$SCAFLD_HOME/.git" ]; then
    echo "  Updating $SCAFLD_HOME"
    git -C "$SCAFLD_HOME" pull --quiet
else
    # If running from curl, clone fresh. If running from existing clone, skip.
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    if [ "$SCRIPT_DIR" = "$SCAFLD_HOME" ]; then
        echo "  Using $SCAFLD_HOME"
    else
        echo "  Cloning to $SCAFLD_HOME"
        git clone --quiet -b main https://github.com/nilstate/scafld.git "$SCAFLD_HOME"
    fi
fi

# Symlink CLI
mkdir -p "$BIN_DIR"
ln -sf "$SCAFLD_HOME/cli/scafld" "$BIN_DIR/scafld"
chmod +x "$SCAFLD_HOME/cli/scafld"
echo "  Linked scafld -> $BIN_DIR/scafld"

# Check PATH
case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo ""
       echo "  Add $BIN_DIR to your PATH:"
       echo "    export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
esac

echo ""
echo "Done! Run 'scafld init' in any project to get started."
