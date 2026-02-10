-- Configuration and constants
local config = {
	-- Audio settings
	SAMPLE_RATE = 44100,
	FRAMES_PER_BUFFER = 512,

	-- Math constants
	PI2 = math.pi * 2,

	-- PortAudio constants
	paFloat32 = 0x00000001,
	paNoFlag = 0,
	paClipOff = 0x00000001,

	-- Synth settings
	MASTER_VOLUME = 0.8,

	-- Default ADSR envelope parameters (attack, decay, sustain, release)
	DEFAULT_ADSR = {
		attack = 0.05, -- 50ms
		decay = 0.1, -- 100ms
		sustain = 0.6, -- 60% level
		release = 0.2, -- 200ms
	},

	-- Note sequence to play
	NOTES = {
		{ freq = 261.63, duration = 333 }, -- C4
		{ freq = 329.63, duration = 333 }, -- E4
		{ freq = 392.00, duration = 333 }, -- G4
		{ freq = 523.25, duration = 1000 }, -- C5
		{ freq = 392.00, duration = 333 }, -- G4
		{ freq = 329.63, duration = 333 }, -- E4
		{ freq = 261.63, duration = 200 }, -- C4
	},
}

return config
