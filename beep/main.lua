#!/usr/bin/env luajit
--[[
  PortAudio ADSR Synthesizer (Blocking I/O version)
  Requires: LuaJIT and libportaudio installed

  macOS: brew install portaudio
  Linux: apt-get install libportaudio2 portaudio19-dev

  Usage: luajit main.lua
]]

local ffi = require("ffi")
local config = require("config")
local Oscillator = require("lib.oscillator")
local ADSR = require("lib.adsr")

-- PortAudio FFI bindings
ffi.cdef[[
  typedef int PaError;
  typedef double PaTime;
  typedef unsigned long PaSampleFormat;
  typedef unsigned long PaStreamFlags;

  typedef struct PaStreamParameters {
    int device;
    int channelCount;
    PaSampleFormat sampleFormat;
    double suggestedLatency;
    void *hostApiSpecificStreamInfo;
  } PaStreamParameters;

  typedef void PaStream;

  PaError Pa_Initialize(void);
  PaError Pa_Terminate(void);
  const char* Pa_GetErrorText(PaError errorCode);
  int Pa_GetDefaultOutputDevice(void);

  PaError Pa_OpenStream(
    PaStream** stream,
    const PaStreamParameters *inputParameters,
    const PaStreamParameters *outputParameters,
    double sampleRate,
    unsigned long framesPerBuffer,
    PaStreamFlags streamFlags,
    void *streamCallback,
    void *userData
  );

  PaError Pa_StartStream(PaStream *stream);
  PaError Pa_StopStream(PaStream *stream);
  PaError Pa_CloseStream(PaStream *stream);
  PaError Pa_WriteStream(PaStream *stream, const void *buffer, unsigned long frames);
]]

-- Load PortAudio library
local pa
if ffi.os == "OSX" then
  pa = ffi.load("portaudio")
elseif ffi.os == "Linux" then
  pa = ffi.load("portaudio.so.2")
else
  pa = ffi.load("portaudio")
end

-- Helper function to check PortAudio errors
local function pa_check(err)
  if err ~= 0 then
    error("PortAudio error: " .. ffi.string(pa.Pa_GetErrorText(err)))
  end
end

-- Generate audio buffer
local function generate_audio(oscillator, envelope, num_frames, master_volume)
  local buffer = ffi.new("float[?]", num_frames * 2)  -- Stereo

  for i = 0, num_frames - 1 do
    local osc_sample = oscillator:generate_sine()
    local env_level = envelope:process()
    local sample = osc_sample * env_level * master_volume

    -- Stereo output
    buffer[i * 2] = sample      -- Left
    buffer[i * 2 + 1] = sample  -- Right
  end

  return buffer
end

-- Main program
local function main()
  print("PortAudio ADSR Synthesizer (Blocking I/O)")
  print("==========================================")

  -- Initialize PortAudio
  pa_check(pa.Pa_Initialize())

  -- Setup output parameters
  local outputParameters = ffi.new("PaStreamParameters")
  outputParameters.device = pa.Pa_GetDefaultOutputDevice()
  outputParameters.channelCount = 2  -- Stereo
  outputParameters.sampleFormat = config.paFloat32
  outputParameters.suggestedLatency = 0.050  -- 50ms
  outputParameters.hostApiSpecificStreamInfo = nil

  -- Open stream (no callback for blocking I/O)
  local stream = ffi.new("PaStream*[1]")
  pa_check(pa.Pa_OpenStream(
    stream,
    nil,  -- No input
    outputParameters,
    config.SAMPLE_RATE,
    config.FRAMES_PER_BUFFER,
    config.paClipOff,
    nil,  -- No callback
    nil   -- No user data
  ))

  -- Start stream
  pa_check(pa.Pa_StartStream(stream[0]))

  print("\nPlaying notes with ADSR envelope...")
  print("Press Ctrl+C to stop\n")

  -- Create synth components
  local oscillator = Oscillator.new(440)
  local envelope = ADSR.new(
    config.DEFAULT_ADSR.attack,
    config.DEFAULT_ADSR.decay,
    config.DEFAULT_ADSR.sustain,
    config.DEFAULT_ADSR.release
  )
  local master_volume = config.MASTER_VOLUME

  -- Play a sequence of notes
  for i, note in ipairs(config.NOTES) do
    print(string.format("Playing %.2f Hz for %d ms", note.freq, note.duration))

    oscillator:set_frequency(note.freq)
    envelope:note_on()

    -- Play note
    local num_buffers = math.ceil((note.duration / 1000) * config.SAMPLE_RATE / config.FRAMES_PER_BUFFER)
    for j = 1, num_buffers do
      local buffer = generate_audio(oscillator, envelope, config.FRAMES_PER_BUFFER, master_volume)
      pa_check(pa.Pa_WriteStream(stream[0], buffer, config.FRAMES_PER_BUFFER))
    end

    envelope:note_off()

    -- Gap between notes (release)
    local gap_buffers = math.ceil(0.2 * config.SAMPLE_RATE / config.FRAMES_PER_BUFFER)
    for j = 1, gap_buffers do
      local buffer = generate_audio(oscillator, envelope, config.FRAMES_PER_BUFFER, master_volume)
      pa_check(pa.Pa_WriteStream(stream[0], buffer, config.FRAMES_PER_BUFFER))
    end
  end

  -- Cleanup
  pa_check(pa.Pa_StopStream(stream[0]))
  pa_check(pa.Pa_CloseStream(stream[0]))
  pa_check(pa.Pa_Terminate())

  print("\nDone!")
end

-- Run with error handling
local success, err = pcall(main)
if not success then
  print("Error: " .. tostring(err))
  pa.Pa_Terminate()
  os.exit(1)
end
