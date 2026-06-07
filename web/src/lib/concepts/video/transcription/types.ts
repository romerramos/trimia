export type Word = {
	word: string;
	punctuatedWord: string;
	start: number;
	end: number;
	confidence: number;
	filler?: boolean;
};
