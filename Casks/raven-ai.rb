cask "raven-ai" do
  version "0.1.5"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"

  # GitHub Releases normalises spaces to dots in asset URLs, so the
  # download path uses "Raven.AI_*", not "Raven AI_*" or the
  # URL-encoded "Raven%20Local_*".
  url "https://github.com/ravencloak-org/Raven/releases/download/raven-ai-v#{version}/Raven.AI_#{version}_universal.dmg",
      verified: "github.com/ravencloak-org/Raven/"
  name "Raven AI"
  desc "Privacy-first desktop edition of Raven that runs locally with Ollama"
  homepage "https://github.com/ravencloak-org/Raven"

  livecheck do
    url :url
    strategy :github_latest do |json|
      json["tag_name"]&.delete_prefix("raven-ai-v")
    end
  end

  # The DMG is not Apple-notarised (free tier — no Developer ID). Homebrew
  # strips the quarantine attribute automatically on `brew install --cask`,
  # so users with Homebrew never hit Gatekeeper. Users installing manually
  # need to run `xattr -d com.apple.quarantine "/Applications/Raven AI.app"`
  # or right-click → Open once. See docs/install/macos.md.
  app "Raven AI.app"

  zap trash: [
    "~/Library/Application Support/io.ravencloak.ai",
    "~/Library/Caches/io.ravencloak.ai",
    "~/Library/Preferences/io.ravencloak.ai.plist",
    "~/Library/Saved Application State/io.ravencloak.ai.savedState",
  ]
end
