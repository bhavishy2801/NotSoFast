"""Render the site's original 8-second evidence sculpture. Build tool only.
Run with Pillow and imageio-ffmpeg installed; the website needs neither.
"""
import math
import random
import subprocess
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter, ImageChops
import imageio_ffmpeg

ROOT = Path(__file__).resolve().parents[1]
W, H, FPS, FRAMES = 1280, 720, 24, 192
rng = random.Random(41)
points = [(rng.uniform(-2.8, 2.8), rng.uniform(-1.9, 1.9), rng.uniform(-1.5, 1.5)) for _ in range(360)]
output = ROOT / 'website/assets/evidence-film.mp4'
encoder = subprocess.Popen([imageio_ffmpeg.get_ffmpeg_exe(), '-y', '-f', 'rawvideo', '-vcodec', 'rawvideo',
    '-s', f'{W}x{H}', '-pix_fmt', 'rgb24', '-r', str(FPS), '-i', '-', '-an', '-c:v', 'libx264',
    '-crf', '21', '-preset', 'fast', '-g', '8', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', str(output)],
    stdin=subprocess.PIPE, stderr=subprocess.DEVNULL)
def smooth(x):
    x = max(0, min(1, x))
    return x*x*(3-2*x)
try:
    for frame in range(FRAMES):
        t = frame/(FRAMES-1)
        image = Image.new('RGB', (W,H), '#080c10')
        light = Image.new('RGB', (W,H))
        glow = ImageDraw.Draw(light)
        glow.ellipse((370,100,910,640), fill=(25,45,34))
        image = ImageChops.add(image, light.filter(ImageFilter.GaussianBlur(100)))
        draw = ImageDraw.Draw(image)
        angle = t*1.6-.8
        def project(x,y,z):
            xx = x*math.cos(angle)+z*math.sin(angle)
            zz = -x*math.sin(angle)+z*math.cos(angle)
            scale = 190/(1+zz*.13)
            return (W/2+xx*scale,H/2+y*scale,scale)
        # Concentric paths act as the instrument's registration rings.
        for ring in range(4):
            path=[]
            for n in range(161):
                a=n/160*math.tau
                x,y,z=(2.1+ring*.15)*math.cos(a), .55*math.sin(a), (1.3+ring*.15)*math.sin(a)
                px,py,_=project(x,y,z)
                path.append((px,py))
            draw.line(path, fill=(34+ring*5,58+ring*4,56+ring*3),width=1)
        morph=smooth((t-.16)/.65)
        projected=[]
        for i,(x,y,z) in enumerate(points):
            a=i*2.399963
            sy=1-2*(i+.5)/len(points)
            radius=math.sqrt(1-sy*sy)
            target=(1.25*radius*math.cos(a),1.25*sy,1.25*radius*math.sin(a))
            px,py,scale=project(x*(1-morph)+target[0]*morph,y*(1-morph)+target[1]*morph,z*(1-morph)+target[2]*morph)
            projected.append((px,py,scale))
        for i,(x,y,scale) in enumerate(projected):
            if morph>.25:
                for j in (i+1,i+13):
                    if j<len(projected):
                        xx,yy,_=projected[j]
                        if math.hypot(x-xx,y-yy)<78:
                            draw.line((x,y,xx,yy),fill=(45,70,58),width=1)
            r=max(1,scale/95)
            bright=int(150+min(1,scale/230)*80)
            draw.ellipse((x-r,y-r,x+r,y+r),fill=(bright,int(bright*.98),int(bright*.65)))
        # Quiet framing marks, no product claims baked into the film.
        for x in (65,W-65):
            draw.line((x-8,H/2,x+8,H/2),fill=(90,110,100))
            draw.line((x,H/2-8,x,H/2+8),fill=(90,110,100))
        if frame==125:image.save(ROOT/'website/assets/evidence-poster.jpg',quality=90)
        encoder.stdin.write(image.tobytes())
finally:
    encoder.stdin.close()
    if encoder.wait()!=0:raise RuntimeError('Film encoder failed')
print(output)
