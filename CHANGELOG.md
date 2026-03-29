# Changelog

## v0.2.0

Major milestone release: GoST now boots the bundled EmuTOS image to the GEM desktop.

Highlights:

- Reach the GEM desktop in both the desktop frontend and headless boot runs
- Improve ST hardware bring-up across MMU, GLUE, Shifter, ACIA/IKBD, MFP, and bus mapping
- Add framebuffer dumping and richer boot tracing for headless debugging
- Add high-resolution monochrome rendering needed for the desktop view
- Reduce default test runtime and move heavy debug probes behind a build tag
- Replace late panic regressions with a desktop-reached regression in the emulator test suite
- Update to upstream `m68kemu v1.2.2` and remove the temporary local `third_party` override

Notes:

- The bundled EmuTOS path now reaches the desktop, but broader real-TOS compatibility is still incomplete
- Several hardware models are still intentionally partial and focused on bring-up rather than full machine accuracy

## v0.1.0

Initial public milestone with the emulator foundation, desktop window, basic ST hardware model, headless mode, tracing, and early EmuTOS bring-up support.
