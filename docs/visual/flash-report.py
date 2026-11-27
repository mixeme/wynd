#!/usr/bin/env python3
"""Разбор записи экрана на мигания: белые кадры, пустые кадры, рывки.

Зачем: жалобы «мигает» приходили записями владельца (C144, C154, C156), и
каждый раз это разбирали заново отдельным скриптом. Здесь один и тот же
разбор: по видео (mp4/mkv/mov) или каталогу PNG считаются покадровые
метрики, события печатаются таблицей, кадры-улики сохраняются рядом с
отчётом. Требуется OpenCV (`pip install opencv-python` в окружении
разработки; ffmpeg не нужен).

    python docs/visual/flash-report.py Record.mp4
    python docs/visual/flash-report.py Record.mp4 --out dev/flash-report
    python docs/visual/flash-report.py dev/screens/shots --out dev/flash-report

Что считается миганием (пороговые значения — по опыту разборов):

- white — кадр почти белый (>= 3 % чистых #FFFFFF): так выглядела вспышка
  при закрытии клавиатуры (C144);
- blank — одноцветный кадр без содержимого (std <= 6): экран «Загрузка…»,
  пустой экран между переходами (C154);
- jump — разом сменилось >= 35 % пикселей: появление/исчезновение диалога,
  клавиатуры, подмена экрана (C156). Это кандидат, а не приговор: смотри
  сохранённые кадры.

События соседних кадров склеиваются (одно закрытие клавиатуры даёт два
импульса — C144). В отчёт пишется пик по каждому событию и три кадра:
за 1 кадр до, в начале и через 0,2 с.
"""
import argparse
import os
import sys

try:
	import cv2
	import numpy as np
except ImportError:  # pragma: no cover - подсказка вместо стека
	sys.exit('нужен OpenCV: pip install opencv-python')


WHITE_MIN = 3.0      # % чистого белого в кадре
BLANK_STD = 6.0      # std серого кадра — одноцветный
JUMP_CHANGED = 35.0  # % изменившихся пикселей между кадрами
JUMP_DELTA = 25      # |Δ| серого, с которого пиксель считается изменившимся
MERGE_SEC = 0.4      # события ближе этого — одно


def frames_from_video(path):
	cap = cv2.VideoCapture(path)
	if not cap.isOpened():
		sys.exit(f'не открывается видео: {path}')
	fps = cap.get(cv2.CAP_PROP_FPS) or 25.0
	i = 0
	while True:
		ok, frame = cap.read()
		if not ok:
			break
		yield i, i / fps, frame
		i += 1


def frames_from_dir(path):
	names = sorted(
		n for n in os.listdir(path)
		if n.lower().endswith(('.png', '.jpg', '.jpeg'))
	)
	if not names:
		sys.exit(f'в каталоге нет PNG/JPG: {path}')
	for i, name in enumerate(names):
		frame = cv2.imread(os.path.join(path, name))
		if frame is None:
			continue
		yield i, float(i), frame


def main():
	ap = argparse.ArgumentParser(description='Разбор записи экрана на мигания')
	ap.add_argument('source', help='видео или каталог с кадрами PNG/JPG')
	ap.add_argument('--out', default='dev/flash-report', help='каталог отчёта и кадров')
	args = ap.parse_args()

	source = args.source
	if not os.path.exists(source):
		sys.exit(f'нет такого пути: {source}')
	os.makedirs(args.out, exist_ok=True)

	events = []
	prev_gray = None
	prev_stamp = 0.0
	count = 0
	size = None

	for idx, stamp, frame in frames_from_video(source) if os.path.isfile(source) else frames_from_dir(source):
		count += 1
		h, w = frame.shape[:2]
		size = (w, h)
		white = float(
			((frame[:, :, 0] >= 250) & (frame[:, :, 1] >= 250) & (frame[:, :, 2] >= 250)).mean() * 100
		)
		gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
		std = float(gray.std())
		changed = 0.0
		if prev_gray is not None and prev_gray.shape == gray.shape:
			diff = cv2.absdiff(gray, prev_gray)
			changed = float((diff > JUMP_DELTA).mean() * 100)
		prev_gray = gray
		prev_stamp = stamp

		kind = None
		value = 0.0
		if white >= WHITE_MIN:
			kind, value = 'white', white
		elif std <= BLANK_STD:
			kind, value = 'blank', std
		elif changed >= JUMP_CHANGED:
			kind, value = 'jump', changed
		if kind:
			events.append({'frame': idx, 't': stamp, 'kind': kind, 'value': value,
						   'white': white, 'std': std, 'changed': changed})

	# Склейка соседних событий одного типа: одно закрытие клавиатуры — два импульса.
	merged = []
	for ev in events:
		last = merged[-1] if merged else None
		if last and ev['kind'] == last['kind'] and ev['t'] - last['end'] <= MERGE_SEC:
			last['end'] = ev['t']
			last['peak'] = max(last['peak'], ev['value'])
			last['frames'].append(ev['frame'])
		else:
			merged.append({'kind': ev['kind'], 'start': ev['t'], 'end': ev['t'],
						   'peak': ev['value'], 'frames': [ev['frame']]})

	# Кадры-улики: начало события и кадр через 0,2 с.
	def grab(base, frames_list):
		start = frames_list[0]
		if not os.path.isfile(source):
			names = sorted(
				n for n in os.listdir(source)
				if n.lower().endswith(('.png', '.jpg', '.jpeg'))
			)
			for suffix, f in (('', start), ('-after', start + 5)):
				if 0 <= f < len(names):
					fr = cv2.imread(os.path.join(source, names[f]))
					if fr is not None:
						cv2.imwrite(base.replace('.png', f'{suffix}.png'), fr)
			return
		cap = cv2.VideoCapture(source)
		if not cap.isOpened():
			return
		fps = cap.get(cv2.CAP_PROP_FPS) or 25.0
		want = {start, start + int(round(0.2 * fps))}
		i = 0
		while want:
			ok, fr = cap.read()
			if not ok:
				break
			if i in want:
				suffix = '' if i == start else '-after'
				cv2.imwrite(base.replace('.png', f'{suffix}.png'), fr)
				want.discard(i)
			i += 1
		cap.release()

	lines = []
	lines.append('# Разбор записи на мигания\n')
	lines.append(f'Источник: `{source}`')
	lines.append(f'Кадров: {count}, длительность: {prev_stamp:.1f} с, размер: {size[0]}×{size[1]} px')
	lines.append(f'Пороги: белое ≥ {WHITE_MIN} %, одноцветный std ≤ {BLANK_STD}, '
				 f'рывок ≥ {JUMP_CHANGED} % изменившихся (|Δ| > {JUMP_DELTA}).\n')
	if not merged:
		lines.append('Событий нет: ни белых кадров, ни пустых, ни рывков больше порога.\n')
	else:
		lines.append('| # | Тип | Время, с | Длит., с | Пик | Кадры-улики |')
		lines.append('|---|---|---|---|---|---|')
	for n, ev in enumerate(merged, 1):
		path = os.path.join(args.out, f'event-{n:02d}-{ev["kind"]}-{ev["start"]:.2f}s.png')
		grab(path, ev['frames'])
		unit = '%' if ev['kind'] != 'blank' else ''
		lines.append(f'| {n} | {ev["kind"]} | {ev["start"]:.2f}–{ev["end"]:.2f} | '
					 f'{ev["end"] - ev["start"]:.2f} | {ev["peak"]:.1f}{unit} | '
					 f'`{os.path.basename(path)}` |')
	lines.append('\n`jump` — кандидат: открой кадры-улики и реши, что это было '
				 '(диалог, клавиатура, подмена экрана). `white` и `blank` — '
				 'дефект, если по сценарию их быть не должно.\n')

	report = os.path.join(args.out, 'report.md')
	with open(report, 'w', encoding='utf-8') as f:
		f.write('\n'.join(lines))
	print('\n'.join(lines))
	print(f'Отчёт: {report}')


if __name__ == '__main__':
	main()
