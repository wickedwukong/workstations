claude-switch-settings() {
  local profile="$1"

  echo "Switching claude settings to ~/.claude/settings.$profile.json"
  unlink "$HOME/.claude/settings.json"
  ln -s "$HOME/.claude/settings.$profile.json" "$HOME/.claude/settings.json"
}
