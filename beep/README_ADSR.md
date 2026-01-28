# PortAudio ADSR Synthesizer

Complete working example of low-level audio synthesis with ADSR envelopes using LuaJIT and PortAudio.

## Installation

**macOS:**
```bash
brew install portaudio luajit
```

**Linux (Debian/Ubuntu):**
```bash
apt-get install libportaudio2 portaudio19-dev luajit
```

**Arch Linux:**
```bash
pacman -S portaudio luajit
```

## Usage

```bash
luajit adsr_synth.lua
```

## Features

### ADSR Envelope
- **Attack**: Time to reach peak level from zero
- **Decay**: Time to fall from peak to sustain level
- **Sustain**: Level held while note is active
- **Release**: Time to fade to silence after note off

### Oscillators
The example includes three waveform types:
- `generate_sine()` - Smooth sine wave
- `generate_square()` - Square wave (change in code)
- `generate_saw()` - Sawtooth wave (change in code)

### Customization

Modify ADSR parameters:
```lua
ADSR.new(attack, decay, sustain, release)
-- Example: Quick pluck sound
ADSR.new(0.001, 0.05, 0.3, 0.1)
-- Example: Pad sound
ADSR.new(0.5, 0.3, 0.7, 1.0)
```

Change waveform in the callback:
```lua
-- Change this line in the audio callback
local osc_sample = synth.oscillator:generate_square()  -- or generate_saw()
```

Add your own note sequence:
```lua
local notes = {
  {freq = 440.00, duration = 500},  -- A4
  {freq = 493.88, duration = 500},  -- B4
  -- Add more notes
}
```

## How It Works

1. **FFI Bindings**: Direct C library access via LuaJIT FFI
2. **Audio Callback**: PortAudio calls your function to fill audio buffers
3. **Sample Generation**: Each frame, oscillator generates waveform
4. **Envelope Application**: ADSR envelope modulates amplitude
5. **Output**: Processed samples sent to audio hardware

## Extending

- Add polyphony (multiple voices)
- Implement filters (low-pass, high-pass)
- Add LFO (low-frequency oscillator) for vibrato/tremolo
- Support MIDI input
- Add effects (reverb, delay)
