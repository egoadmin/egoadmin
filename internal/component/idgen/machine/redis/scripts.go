package redis

const allocateScript = `
local keyPrefix = ARGV[1]
local namespace = ARGV[2]
local instanceID = ARGV[3]
local sessionID = ARGV[4]
local maxMachineID = tonumber(ARGV[5])
local ttlMillis = tonumber(ARGV[6])
local value = instanceID .. "|" .. sessionID

local instanceKey = keyPrefix .. ":" .. namespace .. ":machine:instance:" .. instanceID
local existing = redis.call("GET", instanceKey)
if existing then
  local sep = string.find(existing, "|", 1, true)
  if sep then
    local existingID = tonumber(string.sub(existing, 1, sep - 1))
    local existingSession = string.sub(existing, sep + 1)
    local machineKey = keyPrefix .. ":" .. namespace .. ":machine:id:" .. existingID
    local machineValue = redis.call("GET", machineKey)
    if machineValue == instanceID .. "|" .. existingSession then
      redis.call("PSETEX", machineKey, ttlMillis, value)
      redis.call("PSETEX", instanceKey, ttlMillis, tostring(existingID) .. "|" .. sessionID)
      return { existingID, sessionID }
    end
  end
end

for id = 0, maxMachineID do
  local machineKey = keyPrefix .. ":" .. namespace .. ":machine:id:" .. id
  if redis.call("SET", machineKey, value, "PX", ttlMillis, "NX") then
    redis.call("PSETEX", instanceKey, ttlMillis, tostring(id) .. "|" .. sessionID)
    return { id, sessionID }
  end
end

return { -1, sessionID }
`

const renewScript = `
local keyPrefix = ARGV[1]
local namespace = ARGV[2]
local instanceID = ARGV[3]
local sessionID = ARGV[4]
local machineID = tonumber(ARGV[5])
local ttlMillis = tonumber(ARGV[6])
local value = instanceID .. "|" .. sessionID
local machineKey = keyPrefix .. ":" .. namespace .. ":machine:id:" .. machineID
local instanceKey = keyPrefix .. ":" .. namespace .. ":machine:instance:" .. instanceID

if redis.call("GET", machineKey) == value then
  redis.call("PSETEX", machineKey, ttlMillis, value)
  redis.call("PSETEX", instanceKey, ttlMillis, tostring(machineID) .. "|" .. sessionID)
  return 1
end

return 0
`

const releaseScript = `
local keyPrefix = ARGV[1]
local namespace = ARGV[2]
local instanceID = ARGV[3]
local sessionID = ARGV[4]
local machineID = tonumber(ARGV[5])
local value = instanceID .. "|" .. sessionID
local machineKey = keyPrefix .. ":" .. namespace .. ":machine:id:" .. machineID
local instanceKey = keyPrefix .. ":" .. namespace .. ":machine:instance:" .. instanceID

if redis.call("GET", machineKey) == value then
  redis.call("DEL", machineKey)
  redis.call("DEL", instanceKey)
  return 1
end

return 0
`
