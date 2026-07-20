#! /usr/bin/env bash

[ "$TRACE" = "yes" ] && set -x
set -e
set -o pipefail

source ./lib/loginitems.sh

# atreyu, falkor, bastian, gmork, rockbiter

WORKSTATIONS_NAME=${WORKSTATIONS_NAME:-falkor}

WORKSTATIONS_RUN_SOFTWARE_UPDATE=${WORKSTATIONS_RUN_SOFTWARE_UPDATE:-yes}
WORKSTATIONS_CONFIGURE_SYSTEM=${WORKSTATIONS_CONFIGURE_SYSTEM:-yes}
WORKSTATIONS_CONFIGURE_APPS=${WORKSTATIONS_CONFIGURE_APPS:-yes}

WORKSTATIONS_USER_NAME=${WORKSTATIONS_USER_NAME:-"Toby Clemson"}
WORKSTATIONS_USER_EMAIL=${WORKSTATIONS_USER_EMAIL:-tobyclemson@gmail.com}

uname_machine="$(/usr/bin/uname -m)"
preference_files=()
affected_applications=()

# Ask for the administrator password upfront
sudo -v

# Keep-alive: update existing `sudo` time stamp until script has finished
while true; do sudo -n true; sleep 10; kill -0 "$$" || exit; done 2>/dev/null &

# Install Homebrew prerequisites
if [[ "$WORKSTATIONS_RUN_SOFTWARE_UPDATE" != "no" ]]; then
  softwareupdate --all --install --force
  if [[ "$uname_machine" == "arm64" && ! -e /Library/Apple/usr/share/rosetta/rosetta ]]; then
    softwareupdate --install-rosetta --agree-to-license
  fi
fi

# Install or update Homebrew
if [[ "$uname_machine" == "arm64" ]]; then
  HOMEBREW_PREFIX="/opt/homebrew"
else
  HOMEBREW_PREFIX="/usr/local"
fi
if ! which -s brew; then
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  eval "$($HOMEBREW_PREFIX/bin/brew shellenv)"
else
  brew update
fi

# Upgrade existing brews and casks
brew upgrade

# Install all taps, brews and casks
brew bundle --verbose --file Brewfile

# Clean up before subsequent steps set up ZSH
rm -rf ~/.zshrc.d

# Setup docker
mkdir -p ~/.docker/cli-plugins
cp ./dotfiles/.docker/config.json ~/.docker/config.json
ln -sfn "$HOMEBREW_PREFIX/opt/docker-buildx/bin/docker-buildx" ~/.docker/cli-plugins/docker-buildx
ln -sfn "$HOMEBREW_PREFIX/opt/docker-compose/bin/docker-compose" ~/.docker/cli-plugins/docker-compose
if ! brew services list | grep colima; then
  brew services start colima
fi

# Clean up
brew cleanup

# Un-quarantine casks
# sudo xattr -r -d com.apple.quarantine /Applications/Emacs.app
sudo xattr -r -d com.apple.quarantine /Applications/QLMarkdown.app
sudo xattr -r -d com.apple.quarantine /Applications/Spotify.app

# Reset Quicklook server
qlmanage -r

# Install oh-my-zsh
if [ ! -d "$HOME/.oh-my-zsh/" ]; then
  sh -c "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
fi

# Install prelude
if [ ! -d "$HOME/.emacs.d/" ]; then
  curl -L https://git.io/epre | sh
fi

# Copy SSH config
mkdir -p ~/.ssh
cp ./dotfiles/.ssh/config ~/.ssh

# Copy initialisation dotfiles
cp -R ./dotfiles/.init ~

# Setup oh-my-zsh
cp ./dotfiles/.aliases ~
cp ./dotfiles/.functions ~
cp ./dotfiles/.zprofile ~
cp ./dotfiles/.zshrc ~

# Setup AWS CLI/SDKs
mkdir -p ~/.aws
cp ./dotfiles/.aws/config ~/.aws

mkdir -p ~/.zshrc.d
cp -R ./dotfiles/.zshrc.d/common/* ~/.zshrc.d/

# Setup specific personal zsh configuration
mkdir -p ~/.zshrc.d
cp -R dotfiles/.zshrc.d/personal/* ~/.zshrc.d/

# Setup ebury zsh configuration
mkdir -p ~/.zshrc.d
cp -R dotfiles/.zshrc.d/ebury/* ~/.zshrc.d/

mkdir -p ~/.zsh-completions

# Setup dnsmasq
if [ ! -f "$HOMEBREW_PREFIX/etc/dnsmasq.conf.bak" ]; then
  cp \
    "$HOMEBREW_PREFIX/etc/dnsmasq.conf" \
    "$HOMEBREW_PREFIX/etc/dnsmasq.conf.bak"
fi

cp ./files/etc/dnsmasq.conf "$HOMEBREW_PREFIX/etc/dnsmasq.conf"
cp ./files/etc/dnsmasq.d/localhost.conf \
  "$HOMEBREW_PREFIX/etc/dnsmasq.d/localhost.conf"

sudo brew services restart dnsmasq

# Setup ghostty
cp ./files/home/Library/Application\ Support/com.mitchellh.ghostty/config \
  "$HOME/Library/Application Support/com.mitchellh.ghostty/config"

# Setup Rectangle
cp ./files/home/Library/Preferences/com.knollsoft.Rectangle.plist \
  "$HOME/Library/Preferences/com.knollsoft.Rectangle.plist"

# Store workstation environment variables
if [[ -f "$HOME/.workstation" ]]; then
  rm "$HOME/.workstation"
fi
for var in "${!WORKSTATIONS_@}"; do
  printf 'export %s="%s"\n' "$var" "${!var}" >> "$HOME/.workstation"
done

# Setup prelude
rm -rf ~/.emacs.d/personal
cp -R ./dotfiles/.emacs.d/personal ~/.emacs.d/

# Setup git
cp ./dotfiles/.gitconfig ~
if [[ $(git config --global --get user.name) != *"$WORKSTATIONS_USER_NAME"* ]]; then
  git config --global user.name "$WORKSTATIONS_USER_NAME"
fi
if [[ $(git config --global --get user.email) != *"$WORKSTATIONS_USER_EMAIL"* ]]; then
  git config --global user.email "$WORKSTATIONS_USER_EMAIL"
fi

# Add loginitems
ensure-loginitem "Dropbox" "/Applications/Dropbox.app"
ensure-loginitem "Karabiner-Elements" "/Applications/Karabiner-Elements.app"
ensure-loginitem "Alfred 4" "/Applications/Alfred 4.app"
ensure-loginitem "1Password 7" "/Applications/1Password 7.app"

# Add tool specific config files
cp -R ./dotfiles/.config ~

# Close any open System Settings panes, to prevent them from overriding
# settings we’re about to change
osascript -e "tell application \"System Settings\" to quit"

# Configure system and app preferences
source_preferences () {
  for file in "${preference_files[@]}"; do
    # shellcheck disable=SC1090
    [ -r "$file" ] && [ -f "$file" ] && source "$file"
  done
}

add_system_preferences () {
  preference_files+=("system/$1.sh")
}

add_application_preferences () {
  preference_files+=("apps/$1.sh")
  shift
  affected_applications+=("$@")
}

list_open_affected_applications () {
  local open_applications=()

  # Store the open apps in an array
  for app in "${affected_applications[@]}"; do
    (( $(osascript -e "tell app \"System Events\" to count processes whose name is \"${app}\"") > 0 )) \
      && open_applications+=("$app")
  done

  echo "The following open applications will be affected:"
  printf -- '%s\n' "${open_applications[@]}" | column -x
}

quit_applications () {
  for app in "${affected_applications[@]}"; do
    case "$app" in
      'Quick Look')
        # Restart Quick Look
        qlmanage -r
        ;;
      *)
        killall "$app" &>/dev/null
        # osascript -e "tell application \"${app}\" to quit"
        ;;
    esac
  done
}

# Add system preferences
if [[ "$WORKSTATIONS_CONFIGURE_SYSTEM" != "no" ]]; then
  system_preferences=(
    general
    desktop-screen-saver
    dock
    mission-control
    language-region
    security-privacy
    spotlight
    notifications

    displays
    energy-saver
    keyboard
    # mouse
    trackpad
    printers-scanners
    sound
    # startup-disk

    icloud
    # internet-accounts
    extensions
    app-store
    network
    bluetooth
    sharing

    users-groups
    # parental-controls
    siri
    date-time
    time-machine
    accessibility

    other
    dashboard
    cds-dvds
    ssd
  )

  for preference_pane in "${system_preferences[@]}"; do
    add_system_preferences "$preference_pane"
  done
fi

if [[ "$WORKSTATIONS_CONFIGURE_APPS" != "no" ]]; then
  for app in "cfprefsd" "SystemUIServer" "Dock" "SpeechSynthesisServer"; do
    affected_applications+=("$app")
  done

  # Add Apple application preferences
  add_application_preferences "activity-monitor" "Activity Monitor"
  add_application_preferences "app-store" "App Store"
  add_application_preferences "calendar" "Calendar"
  add_application_preferences "contacts" "Contacts"
  add_application_preferences "disk-utility" "Disk Utility"
  add_application_preferences "finder" "Finder"
  add_application_preferences "font-book" "Font Book"
  add_application_preferences "iwork" "Keynote" "Numbers" "Pages"
  add_application_preferences "mail" "Mail"
  add_application_preferences "messages" "Messages"
  add_application_preferences "photos" "Photos"
  add_application_preferences "quicktime" "QuickTime Player"
  add_application_preferences "safari" "Safari" "WebKit"
  add_application_preferences "screenshot"
  add_application_preferences "terminal"
  add_application_preferences "textedit" "TextEdit"

  # Add 3rd party application preferences
  add_application_preferences "adobe"
  add_application_preferences "dropbox" "Dropbox"
  add_application_preferences "google-chrome" "Google Chrome"
  add_application_preferences "iterm2"

  # Source all preference scripts
  list_open_affected_applications
  source_preferences
  quit_applications
fi
