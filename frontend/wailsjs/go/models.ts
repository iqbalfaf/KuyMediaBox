export namespace config {
	
	export class Output {
	    mode: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new Output(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.dir = source["dir"];
	    }
	}
	export class Settings {
	    outputs: Record<string, Output>;
	    downloadSubfolders: boolean;
	    suffix: string;
	    conflict: string;
	    notify: boolean;
	    skipDownloaded: boolean;
	    autoUpdate: boolean;
	    language: string;
	    toolPaths: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputs = this.convertValues(source["outputs"], Output, true);
	        this.downloadSubfolders = source["downloadSubfolders"];
	        this.suffix = source["suffix"];
	        this.conflict = source["conflict"];
	        this.notify = source["notify"];
	        this.skipDownloaded = source["skipDownloaded"];
	        this.autoUpdate = source["autoUpdate"];
	        this.language = source["language"];
	        this.toolPaths = source["toolPaths"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace downloader {
	
	export class Entry {
	    id: string;
	    url: string;
	    title: string;
	    artist: string;
	    album: string;
	    duration: number;
	    date: string;
	    index: number;
	    thumbnail: string;
	    tab: string;
	    archived: boolean;
	    unavailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.duration = source["duration"];
	        this.date = source["date"];
	        this.index = source["index"];
	        this.thumbnail = source["thumbnail"];
	        this.tab = source["tab"];
	        this.archived = source["archived"];
	        this.unavailable = source["unavailable"];
	    }
	}
	export class Collection {
	    key: string;
	    source: string;
	    type: string;
	    url: string;
	    title: string;
	    subtitle: string;
	    thumbnail: string;
	    entries: Entry[];
	    tabCounts: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.source = source["source"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.thumbnail = source["thumbnail"];
	        this.entries = this.convertValues(source["entries"], Entry);
	        this.tabCounts = source["tabCounts"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Link {
	    source: string;
	    type: string;
	    url: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new Link(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.id = source["id"];
	    }
	}
	export class Options {
	    mode: string;
	    quality: string;
	    container: string;
	    audioFormat: string;
	    audioQuality: string;
	    embed: boolean;
	    skipExisting: boolean;
	    numbering: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.quality = source["quality"];
	        this.container = source["container"];
	        this.audioFormat = source["audioFormat"];
	        this.audioQuality = source["audioQuality"];
	        this.embed = source["embed"];
	        this.skipExisting = source["skipExisting"];
	        this.numbering = source["numbering"];
	    }
	}

}

export namespace imageconv {
	
	export class Options {
	    format: string;
	    quality: number;
	    resizeMode: string;
	    longest: number;
	    percent: number;
	    width: number;
	    height: number;
	    background: string;
	    autoRotate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.quality = source["quality"];
	        this.resizeMode = source["resizeMode"];
	        this.longest = source["longest"];
	        this.percent = source["percent"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.background = source["background"];
	        this.autoRotate = source["autoRotate"];
	    }
	}

}

export namespace main {
	
	export class Capabilities {
	    ffmpeg: boolean;
	    encoders: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ffmpeg = source["ffmpeg"];
	        this.encoders = source["encoders"];
	    }
	}
	export class FileItem {
	    id: string;
	    path: string;
	    name: string;
	    ext: string;
	    size: number;
	    width: number;
	    height: number;
	    duration: number;
	    format: string;
	    videoCodec: string;
	    fps: number;
	    audioCodec: string;
	    sampleRate: number;
	    bitsPerSample: number;
	    channels: number;
	    hasVideo: boolean;
	    hasAudio: boolean;
	    hasCover: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new FileItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.ext = source["ext"];
	        this.size = source["size"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.duration = source["duration"];
	        this.format = source["format"];
	        this.videoCodec = source["videoCodec"];
	        this.fps = source["fps"];
	        this.audioCodec = source["audioCodec"];
	        this.sampleRate = source["sampleRate"];
	        this.bitsPerSample = source["bitsPerSample"];
	        this.channels = source["channels"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.hasCover = source["hasCover"];
	        this.error = source["error"];
	    }
	}
	export class JobItem {
	    id: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new JobItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	    }
	}
	export class JobRef {
	    itemId: string;
	    taskId: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.taskId = source["taskId"];
	    }
	}
	export class VideoJob {
	    mode: string;
	    video: mediaconv.VideoOptions;
	    audio: mediaconv.AudioOptions;
	
	    static createFrom(source: any = {}) {
	        return new VideoJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.video = this.convertValues(source["video"], mediaconv.VideoOptions);
	        this.audio = this.convertValues(source["audio"], mediaconv.AudioOptions);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace mediaconv {
	
	export class AudioOptions {
	    format: string;
	    bitrate: number;
	    channels: string;
	    sampleRate: string;
	    keepMetadata: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AudioOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.bitrate = source["bitrate"];
	        this.channels = source["channels"];
	        this.sampleRate = source["sampleRate"];
	        this.keepMetadata = source["keepMetadata"];
	    }
	}
	export class VideoOptions {
	    format: string;
	    codec: string;
	    quality: string;
	    manual: boolean;
	    crf: number;
	    preset: string;
	    resolution: string;
	    custom: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.codec = source["codec"];
	        this.quality = source["quality"];
	        this.manual = source["manual"];
	        this.crf = source["crf"];
	        this.preset = source["preset"];
	        this.resolution = source["resolution"];
	        this.custom = source["custom"];
	    }
	}

}

export namespace queue {
	
	export class Info {
	    id: string;
	    kind: string;
	    title: string;
	    status: string;
	    progress: number;
	    message: string;
	    detail: string;
	    output: string;
	    outSize: number;
	    started: number;
	    finished: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.output = source["output"];
	        this.outSize = source["outSize"];
	        this.started = source["started"];
	        this.finished = source["finished"];
	    }
	}

}

export namespace tools {
	
	export class Status {
	    id: string;
	    name: string;
	    description: string;
	    found: boolean;
	    path: string;
	    version: string;
	    source: string;
	    runtime: string;
	    latest: string;
	    updateAvailable: boolean;
	    busy: boolean;
	    progress: number;
	    error: string;
	    required: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.found = source["found"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.runtime = source["runtime"];
	        this.latest = source["latest"];
	        this.updateAvailable = source["updateAvailable"];
	        this.busy = source["busy"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	        this.required = source["required"];
	    }
	}

}

export namespace updater {
	
	export class Info {
	    current: string;
	    latest: string;
	    available: boolean;
	    notes: string;
	    url: string;
	    publishedAt: string;
	    mode: string;
	    assetName: string;
	    assetSize: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.notes = source["notes"];
	        this.url = source["url"];
	        this.publishedAt = source["publishedAt"];
	        this.mode = source["mode"];
	        this.assetName = source["assetName"];
	        this.assetSize = source["assetSize"];
	    }
	}

}

