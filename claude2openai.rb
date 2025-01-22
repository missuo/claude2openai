class Claude2openai < Formula
  desc "A proxy to convert Claude API into OpenAI API format"
  homepage "https://github.com/missuo/claude2openai"
  url "https://github.com/missuo/claude2openai.git",
      tag:      "v1.0.3"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w")
  end

  service do
    run [opt_bin/"claude2openai"]
    keep_alive true
    error_log_path var/"log/claude2openai.log"
    log_path var/"log/claude2openai.log"
  end

  test do
    assert_match "Welcome to Claude2OpenAI",
      shell_output("#{bin}/claude2openai --version 2>&1", 1)
  end
end