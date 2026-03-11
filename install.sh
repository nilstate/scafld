#!/usr/bin/env sh
# Trellis installer
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/sourcey/trellis/master/install.sh | sh
#
# Or after cloning:
#   git clone https://github.com/sourcey/trellis.git ~/.trellis
#   ~/.trellis/install.sh
set -e

TRELLIS_HOME="${TRELLIS_HOME:-$HOME/.trellis}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

echo "Installing Trellis..."

# Clone or update
if [ -d "$TRELLIS_HOME/.git" ]; then
    echo "  Updating $TRELLIS_HOME"
    git -C "$TRELLIS_HOME" pull --quiet
else
    # If running from curl, clone fresh. If running from existing clone, skip.
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    if [ "$SCRIPT_DIR" = "$TRELLIS_HOME" ]; then
        echo "  Using $TRELLIS_HOME"
    else
        echo "  Cloning to $TRELLIS_HOME"
        git clone --quiet https://github.com/sourcey/trellis.git "$TRELLIS_HOME"
    fi
fi

# Symlink CLI
mkdir -p "$BIN_DIR"
ln -sf "$TRELLIS_HOME/cli/trellis" "$BIN_DIR/trellis"
chmod +x "$TRELLIS_HOME/cli/trellis"
echo "  Linked trellis -> $BIN_DIR/trellis"

# Check PATH
case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo ""
       echo "  Add $BIN_DIR to your PATH:"
       echo "    export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
esac

echo ""
echo "Done! Run 'trellis init' in any project to get started."
