"""Render the illustrated NotSoFast walkthrough in landscape and portrait.
Build only: Pillow + imageio-ffmpeg. No runtime rendering dependencies.
"""
import subprocess
import argparse
from functools import lru_cache
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont, ImageFilter
import imageio_ffmpeg

ROOT = Path(__file__).resolve().parents[1]
FPS, SECONDS = 30, 12
INK, MUTED, LIME = '#eef3e8', '#a3b0aa', '#d5ff7d'
FONT = ROOT/'website/assets/fonts/SpaceGrotesk.ttf'
BODY = ROOT/'website/assets/fonts/Manrope.ttf'

def ease(t):
    t=max(0,min(1,t))
    return 1-(1-t)**3

@lru_cache(maxsize=32)
def font(size, body=False):
    face=ImageFont.truetype(str(BODY if body else FONT),size)
    face.set_variation_by_axes([500])
    return face

@lru_cache(maxsize=36)
def panel(chapter, progress):
    im=Image.new('RGBA',(1000,670),'#121c19')
    d=ImageDraw.Draw(im)
    def text(x,y,value,size=23,color=INK):d.text((x,y),value,font=font(size),fill=color)
    d.rounded_rectangle((1,1,998,668),radius=22,outline='#52645b',width=2)
    d.line((0,66,1000,66),fill='#33453b',width=1)
    d.rounded_rectangle((25,20,52,47),radius=6,fill=LIME)
    text(32,22,'N',19,'#152119');text(67,22,'NotSoFast',21)
    text(682,26,'SAMPLE / IMMUTABLE SNAPSHOT',12,MUTED)
    text(35,95,'EVIDENCE EXPLORER' if chapter<2 else 'GUARDED PUBLISH',15,LIME)
    text(35,128,['Check the claim.','Search what is missing.','Review before writing.'][chapter],37)
    # File tree and matched witness remain visible through all three shots.
    d.rounded_rectangle((32,202,425,573),radius=14,fill='#0b1210',outline='#2b3c32')
    text(55,227,'sample-repository/',21)
    rows=[('src/',287),('    app.go',334),('config/',400),('    database.yaml',447)]
    for label,y in rows:
        color=INK if chapter or y<390 else '#617369'
        if chapter and y==447:
            d.rounded_rectangle((48,y-7,408,y+37),radius=6,fill='#402c24')
            color='#ffbd95'
        text(56,y,label,22,color)
    text(55,527,'Committed files / source unchanged',14,MUTED)
    d.rounded_rectangle((448,202,968,573),radius=14,fill='#19271f',outline='#3b5040')
    text(472,226,'ABSENCE CLAIM' if chapter<2 else 'PROPOSED WRITE',13,MUTED)
    text(472,257,'database.yaml',30)
    text(472,305,['Scope: src/','Scope: whole repository','New path: database.yaml'][chapter],19,MUTED)
    badge=['UNKNOWN','REFUTED','BLOCKED'][chapter]
    color=['#f1cb7c','#ffbd95','#ffbd95'][chapter]
    d.rounded_rectangle((472,353,700,406),radius=8,fill='#302d21' if chapter==0 else '#402c24')
    text(490,365,badge,23,color)
    text(472,431,['Unsearched folders remain.','Matching witness found.','Duplicate filename detected.'][chapter],22)
    text(472,470,['No whole-repository conclusion.','config/database.yaml','No file written.'][chapter],19,MUTED)
    d.rounded_rectangle((472,518,938,526),radius=4,fill='#354439')
    fill=.28 if chapter==0 else .28+.72*ease(progress*2)
    d.rounded_rectangle((472,518,472+466*fill,526),radius=4,fill=LIME if chapter<2 else '#ffbd95')
    text(34,608,['01  /  Search the selected scope','02  /  Keep the result as evidence','03  /  Policy gate protects the managed branch'][chapter],19,LIME)
    return im

parser=argparse.ArgumentParser()
parser.add_argument('--portrait-only',action='store_true')
args=parser.parse_args()
for portrait in ((True,) if args.portrait_only else (False,True)):
    W,H=(720,900) if portrait else (1440,810)
    name='evidence-film-mobile' if portrait else 'evidence-film'
    out=ROOT/f'website/assets/{name}.mp4'
    temporary=out.with_suffix('.rendering.mp4')
    encoder=subprocess.Popen([imageio_ffmpeg.get_ffmpeg_exe(),'-y','-f','rawvideo','-vcodec','rawvideo','-s',f'{W}x{H}','-pix_fmt','rgb24','-r',str(FPS),'-i','-','-an','-c:v','libx264','-crf','20','-preset','fast','-g','4','-pix_fmt','yuv420p','-movflags','+faststart',str(temporary)],stdin=subprocess.PIPE,stderr=subprocess.DEVNULL)
    try:
        for frame in range(FPS*SECONDS):
            t=frame/FPS
            chapter=min(2,int(t/4))
            local=(t-chapter*4)/4
            im=Image.new('RGB',(W,H),'#080e0b')
            d=ImageDraw.Draw(im)
            # Architectural lighting and a grounded panel, with no orbit/particle imagery.
            for y in range(H):
                glow=max(0,1-abs(y-H*.65)/(H*.6))
                d.line((0,y,W,y),fill=(8+int(glow*6),14+int(glow*10),11+int(glow*6)))
            d.text((36,28),'NOTSOFAST  /  PRODUCT STUDY',font=font(14),fill=MUTED)
            d.text((W-125,28),f'0{chapter+1} / 03',font=font(14),fill=LIME)
            titles=[('A partial search.','An incomplete answer.'),('Find the file.','Change the answer.'),('Stop the duplicate.','Keep the evidence.')]
            size=38 if portrait else 51
            x,y=(36,88) if portrait else (70,112)
            if portrait:
                d.text((x,y),titles[chapter][0],font=font(size),fill=INK)
                d.text((x,y+size+10),titles[chapter][1],font=font(size),fill=LIME)
            card=panel(chapter,min(1,round(local*2,1)))
            if portrait:card=card.crop((448,202,968,573))
            scale=(1.23 if portrait else .76)*(1+.018*ease(local))
            card=card.resize((round(card.width*scale),round(card.height*scale)),Image.Resampling.LANCZOS)
            card=card.rotate(-1.2*(1-ease(local*3)),Image.Resampling.BICUBIC,expand=True)
            # The camera glides into place, then holds long enough to read.
            slide=round(28*(1-ease(local*5)))
            cx=(W-card.width)//2 if portrait else W-card.width-65
            cy=290+slide if portrait else 278+slide
            if not portrait:
                # Landscape keeps the interface central, without text overlapping it.
                cy=215+slide;cx=(W-card.width)//2
                d.rectangle((0,70,W,208),fill='#080e0b')
                d.text((70,103),titles[chapter][0],font=font(42),fill=INK)
                d.text((700,103),titles[chapter][1],font=font(38),fill=LIME)
            shadow=Image.new('RGBA',(W,H));sd=ImageDraw.Draw(shadow)
            sd.rounded_rectangle((cx,cy+15,cx+card.width,cy+card.height+25),radius=24,fill=(0,0,0,150))
            im=Image.alpha_composite(im.convert('RGBA'),shadow.filter(ImageFilter.GaussianBlur(16)))
            im.alpha_composite(card,(cx,cy))
            d=ImageDraw.Draw(im)
            d.text((36,H-35),'ILLUSTRATED WORKFLOW / EXACT FILENAME POLICY',font=font(12),fill=MUTED)
            # Short dip-to-black separates three intentional editorial shots.
            if chapter and local<.065:
                veil=Image.new('RGBA',(W,H),(8,14,11,round(210*(1-ease(local/.065)))))
                im=Image.alpha_composite(im,veil)
            im=im.convert('RGB')
            if frame in (60,180,300):
                im.save(ROOT/f'docs/film-{ "portrait" if portrait else "landscape"}-{chapter+1}.jpg',quality=92)
            if frame==180:im.save(ROOT/f'website/assets/{"evidence-poster-mobile" if portrait else "evidence-poster"}.jpg',quality=92)
            encoder.stdin.write(im.tobytes())
    finally:
        encoder.stdin.close()
        if encoder.wait():raise RuntimeError('Video encoding failed')
    temporary.replace(out)
    print(out)
