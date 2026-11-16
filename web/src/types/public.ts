export type PublicFile = {
	id: string;
	original_filename: string;
	size_bytes: number;
	media_type?: string;
	description?: string;
	created_at: string;
};

export type PublicArtifact = {
	id: string;
	title: string;
	creator_name?: string;
	source_url: string;
	description?: string;
	created_at: string;
	files: PublicFile[];
};

export type PublicFileDetail = PublicFile & { artifact_id: string; artifact_title: string };
