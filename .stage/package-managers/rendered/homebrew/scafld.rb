class Scafld < Formula
  desc "Deterministic protocol for multi-phase agent work"
  homepage "https://0state.com/scafld"
  version "2.5.8"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/nilstate/scafld/releases/download/v2.5.8/scafld_2.5.8_darwin_arm64"
      sha256 "e1461dd6014550aacccb5f4fdf9298f908124a079517da074ae9cea5de5b51ff"
    else
      url "https://github.com/nilstate/scafld/releases/download/v2.5.8/scafld_2.5.8_darwin_amd64"
      sha256 "518b92d5e6786e69e7bc4abb34b5f5bda71083b2c007fa92511a19c352f8a0a3"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/nilstate/scafld/releases/download/v2.5.8/scafld_2.5.8_linux_arm64"
      sha256 "546e0a3ebeed1893ff87f856b590e71305f41c51375f6a753d3a9e6cdd7bf5e2"
    else
      url "https://github.com/nilstate/scafld/releases/download/v2.5.8/scafld_2.5.8_linux_amd64"
      sha256 "b0f50a08ae47c9a67729a1c412937c2d25f50e901a8ac6671862594b765481d8"
    end
  end

  def install
    bin.install Dir["scafld_*"].first => "scafld"
    chmod 0755, bin/"scafld"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/scafld --version")
  end
end
