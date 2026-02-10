-- PortAudio FFI bindings
local ffi = require("ffi")

-- Define PortAudio C API
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

-- Load PortAudio library (platform-specific)
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

-- Export module
return {
  pa = pa,
  pa_check = pa_check,
  ffi = ffi
}
