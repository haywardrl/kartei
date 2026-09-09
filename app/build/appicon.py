"""Draw the Kartei app icon: a 32x32 pixel slip box on a rounded dark square,
scaled to 1024x1024 with hard pixels. Pure Python; writes a PNG by hand.

    python3 app/build/appicon.py app/build/appicon.png   # then wails build"""
import struct, zlib, sys

N = 32
P = {  # Endesga 32, the room's palette
    'wall': (58, 42, 48), 'wall2': (46, 33, 38), 'body': (106, 60, 41), 'bodyD': (92, 50, 35), 'bodyL': (122, 70, 48),
    'lid': (143, 86, 55), 'lidL': (184, 116, 74), 'front': (143, 86, 55), 'frontL': (166, 103, 63), 'frontD': (124, 72, 48),
    'edge': (62, 33, 25), 'cream': (234, 212, 170), 'ink': (24, 20, 37), 'gold': (254, 174, 52), 'yellow': (254, 231, 97),
    'amber': (247, 118, 34), 'shadow': (38, 27, 32),
}
px = [[None] * N for _ in range(N)]  # None = transparent

def rect(x, y, w, h, c):
    for j in range(y, y + h):
        for i in range(x, x + w):
            if 0 <= i < N and 0 <= j < N:
                px[j][i] = P[c]

# rounded background square, radius 5
R = 5
for j in range(N):
    for i in range(N):
        cx = min(i, N - 1 - i); cy = min(j, N - 1 - j)
        if cx < R and cy < R and (R - 1 - cx) ** 2 + (R - 1 - cy) ** 2 > (R - 1) ** 2 + 1:
            continue
        px[j][i] = P['wall']
for i in range(0, N, 8):  # panelling lines, very faint
    for j in range(N):
        if px[j][i] is not None: px[j][i] = P['wall2']

# the box: 22 wide, 24 tall, centred, with a shadow to the lower right
BX, BY, BW, BH = 5, 4, 22, 24
rect(BX + 2, BY + 2, BW, BH, 'shadow')
rect(BX, BY, BW, BH, 'body')
rect(BX, BY, BW, 3, 'lid'); rect(BX, BY, BW, 1, 'lidL')
rect(BX, BY, 1, BH, 'bodyL'); rect(BX + BW - 1, BY, 1, BH, 'bodyD')
rect(BX, BY + BH - 2, BW, 2, 'edge')
# two columns, three rows of drawer fronts
cw, rh = 9, 6
for row in range(3):
    for col in range(2):
        x = BX + 2 + col * (cw + 1); y = BY + 4 + row * (rh + 1)
        rect(x, y, cw, rh, 'front'); rect(x, y, cw, 1, 'frontL'); rect(x, y + rh - 1, cw, 1, 'edge'); rect(x, y, 1, rh, 'frontL')
        rect(x + 1, y + 2, 3, 2, 'cream')          # number plate
        rect(x + cw - 3, y + 2, 2, 2, 'gold'); rect(x + cw - 3, y + 2, 2, 1, 'yellow')  # pull
# a glint of lamplight on the top-left of the lid
rect(BX + 1, BY, 6, 1, 'yellow')

S = 1024 // N
def row_bytes(j):
    out = bytearray([0])  # filter type 0
    for i in range(N):
        c = px[j][i]
        out += (bytes(c) + b'\xff' if c else b'\x00\x00\x00\x00') * S
    return bytes(out)
raw = b''.join(row_bytes(j) for j in range(N) for _ in range(S))
def chunk(t, d): return struct.pack('>I', len(d)) + t + d + struct.pack('>I', zlib.crc32(t + d) & 0xffffffff)
png = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', N * S, N * S, 8, 6, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(raw, 9)) + chunk(b'IEND', b'')
open(sys.argv[1], 'wb').write(png)
print('wrote', sys.argv[1], N * S, 'px')
