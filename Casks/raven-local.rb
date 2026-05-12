cask "raven-local" do
  version "0.1.4"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"

  # GitHub Releases normalises spaces to dots in asset URLs, so the
  # download path uses "Raven.Local_*", not "Raven Local_*" or the
  # URL-encoded "Raven%20Local_*".
  url "https://github.com/ravencloak-org/Raven/releases/download/raven-local-v#{version}/Raven.Local_#{version}_universal.dmg",
      verified: "github.com/ravencloak-org/Raven/"
  name "Raven Local"
  desc "Privacy-first desktop edition of Raven that runs locally with Ollama"
  homepage "https://github.com/ravencloak-org/Raven"

  livecheck do
    url :url
    strategy :github_latest do |json|
      json["tag_name"]&.delete_prefix("raven-local-v")
    end
  end

  # The DMG is not Apple-notarised (free tier — no Developer ID). Homebrew
  # strips the quarantine attribute automatically on `brew install --cask`,
  # so users with Homebrew never hit Gatekeeper. Users installing manually
  # need to run `xattr -d com.apple.quarantine "/Applications/Raven Local.app"`
  # or right-click → Open once. See docs/install/macos.md.
  app "Raven Local.app"

  zap trash: [
    "~/Library/Application Support/io.ravencloak.local",
    "~/Library/Caches/io.ravencloak.local",
    "~/Library/Preferences/io.ravencloak.local.plist",
    "~/Library/Saved Application State/io.ravencloak.local.savedState",
  ]
end
