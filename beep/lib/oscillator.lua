-- Simple Oscillator
local config = require("config")

local Oscillator = {}
Oscillator.__index = Oscillator

function Oscillator.new(frequency)
  local self = setmetatable({}, Oscillator)
  self.frequency = frequency or 440.0  -- A4
  self.phase = 0.0
  self.sample_rate = config.SAMPLE_RATE
  self.PI2 = config.PI2
  return self
end

function Oscillator:set_frequency(freq)
  self.frequency = freq
end

function Oscillator:generate_sine()
  local sample = math.sin(self.phase)
  self.phase = self.phase + (self.PI2 * self.frequency / self.sample_rate)

  -- Wrap phase to prevent floating point drift
  if self.phase >= self.PI2 then
    self.phase = self.phase - self.PI2
  end

  return sample
end

function Oscillator:generate_square()
  local sample = self.phase < math.pi and 1.0 or -1.0
  self.phase = self.phase + (self.PI2 * self.frequency / self.sample_rate)

  if self.phase >= self.PI2 then
    self.phase = self.phase - self.PI2
  end

  return sample
end

function Oscillator:generate_saw()
  local sample = (self.phase / math.pi) - 1.0
  self.phase = self.phase + (self.PI2 * self.frequency / self.sample_rate)

  if self.phase >= self.PI2 then
    self.phase = self.phase - self.PI2
  end

  return sample
end

return Oscillator
