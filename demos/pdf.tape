# PDF preview walkthrough for docs/assets/pdf-preview.gif. Run through `just demo-pdf`,
# which writes tmp/pdf-demo.env pointing HOME at a disposable copy of the demo collection.
# VHS's terminal does not draw Kitty graphics, so this records --renderer=cells:
# page pictures appear as color cells, not native images.
Output docs/assets/pdf-preview.gif

Require peep
Require pdftoppm

Set Shell bash
Set FontSize 15
Set Width 1240
Set Height 760
Set Padding 16
Set Framerate 12
Set TypingSpeed 60ms

Hide
Type `source tmp/pdf-demo.env && cd "$HOME/night-shift" && clear`
Enter
Sleep 500ms
Show

Type "peep --renderer=cells"
Sleep 400ms
Enter
Sleep 1.5s
Type "/07"
Sleep 600ms
Enter
Sleep 400ms
Enter
Sleep 2.5s

Enter
Sleep 1.5s
Right
Sleep 2s

Type "R"
Sleep 2.5s
Left
Sleep 2.5s
Type "3G"
Sleep 2.5s

Type "R"
Sleep 1.5s
Type "/antenna"
Sleep 1s
Enter
Sleep 2.5s

Type "q"
Sleep 500ms
