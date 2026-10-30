{
  lib,
  buildGoModule,
  makeWrapper,
  yt-dlp,
  mpv,
  ffmpeg,
  chafa,
}:
buildGoModule {
  pname = "yt-x";
  version = "1.0.1";
  src = lib.cleanSource ./.;

  # After changing go.mod/go.sum, set this to lib.fakeHash, run `nix build`,
  # and paste the hash from the error message.
  vendorHash = lib.fakeHash;

  subPackages = [ "cmd/yt-x" ];
  ldflags = [ "-s" "-w" "-X github.com/jkpair/yt-x-plus/internal/app.Version=1.0.1" ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/yt-x \
      --prefix PATH : ${lib.makeBinPath [ yt-dlp mpv ffmpeg chafa ]}
    install -Dm644 plugins/example.lua -t $out/share/yt-x/plugins
  '';

  meta = {
    description = "Browse YouTube from your terminal";
    homepage = "https://github.com/jkpair/yt-x-plus";
    license = lib.licenses.mit;
    mainProgram = "yt-x";
  };
}
