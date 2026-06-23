-- KEYS[1]: inventory key (e.g. inventory:product:1)
-- ARGV[1]: stream key (e.g. sales:orders)
-- ARGV[2]: order JSON string

local stock = redis.call('DECR', KEYS[1])
if stock < 0 then
    redis.call('INCR', KEYS[1])
    return {-2, ''}
end

-- Attempt to add the order to the stream.
-- Use pcall to catch any Redis error.
local ok, msg_id = pcall(function()
    return redis.call('XADD', ARGV[1], '*', 'order', ARGV[2])
end)

if not ok then
    -- Rollback the decrement if XADD failed.
    redis.call('INCR', KEYS[1])
    return {-1, ''}
end

return {stock, msg_id}