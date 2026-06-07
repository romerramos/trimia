import type * as segments from '$lib/concepts/video/segments';
import type { TranscriptWord } from './types';

export function wordsFromSegments(sourceSegments: segments.Segment[]) {
	return sourceSegments.flatMap((segment) => {
		return (segment.words ?? []).map((word, index) => ({
			id: `${segment.id}-${index}-${word.start}`,
			text: word.punctuatedWord || word.word,
			start: word.start,
			end: word.end,
			included: segment.included
		})) satisfies TranscriptWord[];
	});
}

export function activeWordId(words: TranscriptWord[], sourceTime: number) {
	return words.find((word) => sourceTime >= word.start && sourceTime < word.end)?.id ?? '';
}
