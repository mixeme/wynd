export type CircleColor =
	| 'terracotta'
	| 'teal'
	| 'olive'
	| 'ochre'
	| 'plum'
	| 'indigo'
	| 'coffee'
	| 'slate';

export type CircleTheme = {
	hex: string;
	cssVar: string;
	tint: string;
};

export const CIRCLE_COLORS: Record<CircleColor, CircleTheme> = {
	terracotta: { hex: '#AF5839', cssVar: 'var(--terracotta)', tint: '#F1E1DA' },
	teal: { hex: '#357077', cssVar: 'var(--teal)', tint: '#DCE8E9' },
	olive: { hex: '#58673A', cssVar: 'var(--olive)', tint: '#E3E7D9' },
	ochre: { hex: '#70571B', cssVar: 'var(--ochre)', tint: '#ECE5D1' },
	plum: { hex: '#7A4265', cssVar: 'var(--plum)', tint: '#EEDFE7' },
	indigo: { hex: '#3C4D83', cssVar: 'var(--indigo)', tint: '#DEE2EE' },
	coffee: { hex: '#62452F', cssVar: 'var(--coffee)', tint: '#E8DFD7' },
	slate: { hex: '#3D494F', cssVar: 'var(--slate)', tint: '#DFE3E5' }
};

export const CIRCLE_COLOR_ORDER: CircleColor[] = [
	'terracotta',
	'teal',
	'olive',
	'ochre',
	'plum',
	'indigo',
	'coffee',
	'slate'
];
