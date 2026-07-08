Output docs/assets/serve.gif

Set FontSize 16
Set Width 1200
Set Height 480
Set Theme "Catppuccin Mocha"
Set Shell bash

Type "# Start REST API + web UI"
Enter
Sleep 300ms
Type "genesisrk serve --port=8080 --static=frontend/dist"
Enter
Sleep 300ms
Type "# Open http://localhost:8080"
Enter
Sleep 2s
