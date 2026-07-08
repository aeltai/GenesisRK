Output docs/assets/generate.gif

Set FontSize 16
Set Width 1200
Set Height 520
Set Theme "Catppuccin Mocha"
Set Shell bash

Type "# Declarative: generate from YAML config"
Enter
Sleep 300ms
Type "genesisrk generate --rancher=v2.13.1 --config=config.example.yaml -o v2.13.1-images.txt"
Enter
Sleep 300ms
Type "# Optional Hauler manifest"
Enter
Sleep 200ms
Type "genesisrk generate --rancher=v2.13.1 --config=config.example.yaml --hauler"
Enter
Sleep 2s
