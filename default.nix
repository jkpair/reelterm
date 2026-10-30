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
  pname = "reelterm";
  version = "1.1.1";
  src = lib.cleanSource ./.;

  # After changing go.mod/go.sum, set this to lib.fakeHash, run `nix build`,
  # and paste the hash from the error message.
  vendorHash = lib.fakeHash;

  subPackages = [ "cmd/reelterm" ];
  ldflags = [ "-s" "-w" "-X github.com/jkpair/reelterm/internal/app.Version=1.1.1" ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/reelterm \
      --prefix PATH : ${lib.makeBinPath [ yt-dlp mpv ffmpeg chafa ]}
    install -Dm644 plugins/example.lua -t $out/share/reelterm/plugins
  '';

  meta = {
    description = "Browse YouTube from your terminal";
    homepage = "https://github.com/jkpair/reelterm";
    license = lib.licenses.mit;
    mainProgram = "reelterm";
  };
}
