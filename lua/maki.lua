-- maki

if (x == 0) and (y == 0) then
    return red
end

if (math.abs(x) == 2) or (math.abs(y) == 2) then
    return green
end

return white
