{
  description = "Browse YouTube from your terminal with reelterm";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, self, ... }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
      runtimeDeps = with pkgs; [
        yt-dlp
        mpv
        ffmpeg
        chafa
      ];
    in
    {
      packages = {
        default = self.packages.${system}.reelterm;
        reelterm = pkgs.callPackage ./default.nix { };
      };

      devShells.default = pkgs.mkShell {
        packages = runtimeDeps ++ (with pkgs; [
          go
          gopls
          golangci-lint
          goreleaser
        ]);
      };
    });
}
