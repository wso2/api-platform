-- GCRA Rate Limiter Lua Script (Atomic)
-- KEYS[1]: rate limit key
-- ARGV[1]: now (nanoseconds)
-- ARGV[2]: emission_interval (nanoseconds)
-- ARGV[3]: burst_allowance (nanoseconds)
-- ARGV[4]: burst_capacity
-- ARGV[5]: expiration (seconds)
-- ARGV[6]: count (number of requests) - NEW!

local key = KEYS[1]
local now = tonumber(ARGV[1])
local emission_interval = tonumber(ARGV[2])
local burst_allowance = tonumber(ARGV[3])
local burst_capacity = tonumber(ARGV[4])
local expiration = tonumber(ARGV[5])
local count = tonumber(ARGV[6]) or 1  -- NEW: default to 1 if not provided

-- Get current TAT (Theoretical Arrival Time)
local tat = redis.call('GET', key)

if tat == false then
    -- First request - TAT = now
    tat = now
else
    tat = tonumber(tat)
end

-- GCRA algorithm: TAT = max(TAT, now)
if tat < now then
    tat = now
end

-- Calculate earliest allowed time
local allow_at = tat - burst_allowance

-- Check if request is allowed
local allowed = 0
local new_tat = tat
local retry_after_nanos = 0

-- Calculate remaining capacity BEFORE consuming
local remaining = burst_capacity
local used_burst = tat - now
if used_burst > 0 and used_burst <= burst_allowance then
    remaining = burst_capacity - math.ceil(used_burst / emission_interval)
    if remaining < 0 then
        remaining = 0
    end
end

if now >= allow_at and count <= remaining then
    -- Request allowed (time check AND capacity check)
    allowed = 1
    new_tat = tat + (emission_interval * count)  -- MODIFIED: multiply by count

    -- Update TAT in Redis with expiration (skip for peek operations where count=0)
    if count > 0 then
        redis.call('SET', key, new_tat, 'EX', expiration)
    end

    -- Recalculate remaining after consuming
    if new_tat < now then
        remaining = burst_capacity
    else
        local used_burst = new_tat - now
        if used_burst <= burst_allowance then
            remaining = burst_capacity - math.ceil(used_burst / emission_interval)
            if remaining < 0 then
                remaining = 0
            end
        else
            remaining = 0
        end
    end
else
    -- Request denied
    retry_after_nanos = allow_at - now
    if retry_after_nanos < 0 then
        retry_after_nanos = 0
    end
end

-- Calculate full quota available time
-- Full quota is available when TAT <= now (all tokens regenerated)
local full_quota_at_nanos = new_tat
if new_tat < now then
    full_quota_at_nanos = now
end

-- Return: {allowed, remaining, reset_nanos, retry_after_nanos, full_quota_at_nanos}
return {allowed, remaining, new_tat, retry_after_nanos, full_quota_at_nanos}
