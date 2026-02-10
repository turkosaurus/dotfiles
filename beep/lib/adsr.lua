-- ADSR Envelope Generator
local config = require("config")

local ADSR = {}
ADSR.__index = ADSR

function ADSR.new(attack, decay, sustain, release)
  local self = setmetatable({}, ADSR)
  self.attack_time = attack or 0.1    -- seconds
  self.decay_time = decay or 0.1      -- seconds
  self.sustain_level = sustain or 0.7 -- 0-1
  self.release_time = release or 0.3  -- seconds

  self.state = "idle"  -- idle, attack, decay, sustain, release
  self.current_level = 0.0
  self.sample_rate = config.SAMPLE_RATE
  self.time = 0

  return self
end

function ADSR:note_on()
  self.state = "attack"
  self.time = 0
end

function ADSR:note_off()
  if self.state ~= "idle" then
    self.state = "release"
    self.time = 0
  end
end

function ADSR:process()
  if self.state == "idle" then
    self.current_level = 0.0

  elseif self.state == "attack" then
    self.time = self.time + 1
    local progress = self.time / (self.attack_time * self.sample_rate)
    self.current_level = progress

    if progress >= 1.0 then
      self.current_level = 1.0
      self.state = "decay"
      self.time = 0
    end

  elseif self.state == "decay" then
    self.time = self.time + 1
    local progress = self.time / (self.decay_time * self.sample_rate)
    self.current_level = 1.0 - (progress * (1.0 - self.sustain_level))

    if progress >= 1.0 then
      self.current_level = self.sustain_level
      self.state = "sustain"
      self.time = 0
    end

  elseif self.state == "sustain" then
    self.current_level = self.sustain_level

  elseif self.state == "release" then
    self.time = self.time + 1
    local start_level = self.current_level
    local progress = self.time / (self.release_time * self.sample_rate)
    self.current_level = start_level * (1.0 - progress)

    if progress >= 1.0 then
      self.current_level = 0.0
      self.state = "idle"
      self.time = 0
    end
  end

  return self.current_level
end

function ADSR:is_active()
  return self.state ~= "idle"
end

return ADSR
