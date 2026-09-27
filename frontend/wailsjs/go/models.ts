export namespace config {
	
	export class FlowStep {
	    kind: string;
	    tool: string;
	    preset: string;
	    label: string;
	    job: number[];
	
	    static createFrom(source: any = {}) {
	        return new FlowStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.tool = source["tool"];
	        this.preset = source["preset"];
	        this.label = source["label"];
	        this.job = source["job"];
	    }
	}
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
	export class Workflow {
	    id: string;
	    name: string;
	    steps: FlowStep[];
	
	    static createFrom(source: any = {}) {
	        return new Workflow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.steps = this.convertValues(source["steps"], FlowStep);
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
	export class WatchRule {
	    id: string;
	    dir: string;
	    kind: string;
	    options: number[];
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WatchRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.dir = source["dir"];
	        this.kind = source["kind"];
	        this.options = source["options"];
	        this.enabled = source["enabled"];
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
	    theme: string;
	    parallel: Record<string, number>;
	    cookiesBrowser: string;
	    cookiesFile: string;
	    nameTemplate: string;
	    spotifyTemplate: string;
	    spotifyLogin: boolean;
	    clipboardWatch: boolean;
	    tray: boolean;
	    downloadLimitKB: number;
	    watch: WatchRule[];
	    workflows: Workflow[];
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
	        this.theme = source["theme"];
	        this.parallel = source["parallel"];
	        this.cookiesBrowser = source["cookiesBrowser"];
	        this.cookiesFile = source["cookiesFile"];
	        this.nameTemplate = source["nameTemplate"];
	        this.spotifyTemplate = source["spotifyTemplate"];
	        this.spotifyLogin = source["spotifyLogin"];
	        this.clipboardWatch = source["clipboardWatch"];
	        this.tray = source["tray"];
	        this.downloadLimitKB = source["downloadLimitKB"];
	        this.watch = this.convertValues(source["watch"], WatchRule);
	        this.workflows = this.convertValues(source["workflows"], Workflow);
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
	    kind: string;
	    archived: boolean;
	    unavailable: boolean;
	    source: string;
	
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
	        this.kind = source["kind"];
	        this.archived = source["archived"];
	        this.unavailable = source["unavailable"];
	        this.source = source["source"];
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
	    photo: boolean;
	    short: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Link(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.id = source["id"];
	        this.photo = source["photo"];
	        this.short = source["short"];
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
	    imageFormat: string;
	    subtitles: string;
	    subLangs: string;
	    sectionStart: string;
	    sectionEnd: string;
	    sponsorBlock: string;
	    playlist: boolean;
	    lyrics: boolean;
	    workflow: string;
	    outDir: string;
	
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
	        this.imageFormat = source["imageFormat"];
	        this.subtitles = source["subtitles"];
	        this.subLangs = source["subLangs"];
	        this.sectionStart = source["sectionStart"];
	        this.sectionEnd = source["sectionEnd"];
	        this.sponsorBlock = source["sponsorBlock"];
	        this.playlist = source["playlist"];
	        this.lyrics = source["lyrics"];
	        this.workflow = source["workflow"];
	        this.outDir = source["outDir"];
	    }
	}

}

export namespace history {
	
	export class Entry {
	    id: string;
	    time: number;
	    started: number;
	    kind: string;
	    title: string;
	    input: string;
	    output: string;
	    inSize: number;
	    outSize: number;
	    status: string;
	    message: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.started = source["started"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.inSize = source["inSize"];
	        this.outSize = source["outSize"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	    }
	}

}

export namespace imageconv {
	
	export class Box {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Box(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class CollageOptions {
	    cols: number;
	    width: number;
	    gap: number;
	    cell: string;
	    fit: string;
	    background: string;
	    radius: number;
	
	    static createFrom(source: any = {}) {
	        return new CollageOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cols = source["cols"];
	        this.width = source["width"];
	        this.gap = source["gap"];
	        this.cell = source["cell"];
	        this.fit = source["fit"];
	        this.background = source["background"];
	        this.radius = source["radius"];
	    }
	}
	export class Watermark {
	    enabled: boolean;
	    type: string;
	    text: string;
	    bold: boolean;
	    color: string;
	    image: string;
	    size: number;
	    opacity: number;
	    angle: number;
	    position: string;
	    margin: number;
	
	    static createFrom(source: any = {}) {
	        return new Watermark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.type = source["type"];
	        this.text = source["text"];
	        this.bold = source["bold"];
	        this.color = source["color"];
	        this.image = source["image"];
	        this.size = source["size"];
	        this.opacity = source["opacity"];
	        this.angle = source["angle"];
	        this.position = source["position"];
	        this.margin = source["margin"];
	    }
	}
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
	    keepMetadata: boolean;
	    targetKB: number;
	    icoSizes: number[];
	    rotate: number;
	    flipH: boolean;
	    flipV: boolean;
	    crop: string;
	    cropBox: Box;
	    watermark: Watermark;
	    aiUpscale: number;
	    aiModel: string;
	    removeBg: boolean;
	    bgModel: string;
	    bgMask: boolean;
	    pngCompress: string;
	
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
	        this.keepMetadata = source["keepMetadata"];
	        this.targetKB = source["targetKB"];
	        this.icoSizes = source["icoSizes"];
	        this.rotate = source["rotate"];
	        this.flipH = source["flipH"];
	        this.flipV = source["flipV"];
	        this.crop = source["crop"];
	        this.cropBox = this.convertValues(source["cropBox"], Box);
	        this.watermark = this.convertValues(source["watermark"], Watermark);
	        this.aiUpscale = source["aiUpscale"];
	        this.aiModel = source["aiModel"];
	        this.removeBg = source["removeBg"];
	        this.bgModel = source["bgModel"];
	        this.bgMask = source["bgMask"];
	        this.pngCompress = source["pngCompress"];
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

export namespace main {
	
	export class AfterQueue {
	    action: string;
	    pending: boolean;
	    seconds: number;
	
	    static createFrom(source: any = {}) {
	        return new AfterQueue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.pending = source["pending"];
	        this.seconds = source["seconds"];
	    }
	}
	export class AudioJob {
	    mode: string;
	    options: mediaconv.AudioOptions;
	
	    static createFrom(source: any = {}) {
	        return new AudioJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.options = this.convertValues(source["options"], mediaconv.AudioOptions);
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
	export class CLICommand {
	    installed: boolean;
	    path: string;
	    onPath: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CLICommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.path = source["path"];
	        this.onPath = source["onPath"];
	    }
	}
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
	export class CollageJob {
	    layout: imageconv.CollageOptions;
	    format: string;
	    quality: number;
	
	    static createFrom(source: any = {}) {
	        return new CollageJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.layout = this.convertValues(source["layout"], imageconv.CollageOptions);
	        this.format = source["format"];
	        this.quality = source["quality"];
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
	export class EditItem {
	    kind: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    x2: number;
	    y2: number;
	    points: number[][];
	    text: string;
	    size: number;
	    bold: boolean;
	    align: string;
	    color: string;
	    fill: string;
	    stroke: number;
	    opacity: number;
	    angle: number;
	    page: number;
	    imageUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new EditItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.x2 = source["x2"];
	        this.y2 = source["y2"];
	        this.points = source["points"];
	        this.text = source["text"];
	        this.size = source["size"];
	        this.bold = source["bold"];
	        this.align = source["align"];
	        this.color = source["color"];
	        this.fill = source["fill"];
	        this.stroke = source["stroke"];
	        this.opacity = source["opacity"];
	        this.angle = source["angle"];
	        this.page = source["page"];
	        this.imageUrl = source["imageUrl"];
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
	    pages: number;
	    encrypted: boolean;
	    locked: boolean;
	    subCodec: string;
	    subFile: string;
	    tags: Record<string, string>;
	    cue: string;
	    cueTracks: number;
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
	        this.pages = source["pages"];
	        this.encrypted = source["encrypted"];
	        this.locked = source["locked"];
	        this.subCodec = source["subCodec"];
	        this.subFile = source["subFile"];
	        this.tags = source["tags"];
	        this.cue = source["cue"];
	        this.cueTracks = source["cueTracks"];
	        this.error = source["error"];
	    }
	}
	export class ImportResult {
	    ui: Record<string, string>;
	    presets: number;
	    workflows: number;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ui = source["ui"];
	        this.presets = source["presets"];
	        this.workflows = source["workflows"];
	        this.version = source["version"];
	    }
	}
	export class JobItem {
	    id: string;
	    path: string;
	    tags?: mediaconv.Tags;
	    outDir?: string;
	    plain?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JobItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.tags = this.convertValues(source["tags"], mediaconv.Tags);
	        this.outDir = source["outDir"];
	        this.plain = source["plain"];
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
	export class ModelStatus {
	    kind: string;
	    id: string;
	    sizeMB: number;
	    installed: boolean;
	    busy: boolean;
	    progress: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.sizeMB = source["sizeMB"];
	        this.installed = source["installed"];
	        this.busy = source["busy"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	    }
	}
	export class OpenRequest {
	    page: string;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new OpenRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.paths = source["paths"];
	    }
	}
	export class PdfEditRequest {
	    tool: string;
	    sources: pdf.Input[];
	    pages: pdf.PageRef[];
	    items: EditItem[];
	    redact: pdf.RedactOptions;
	    crop: pdf.CropOptions;
	
	    static createFrom(source: any = {}) {
	        return new PdfEditRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.sources = this.convertValues(source["sources"], pdf.Input);
	        this.pages = this.convertValues(source["pages"], pdf.PageRef);
	        this.items = this.convertValues(source["items"], EditItem);
	        this.redact = this.convertValues(source["redact"], pdf.RedactOptions);
	        this.crop = this.convertValues(source["crop"], pdf.CropOptions);
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
	export class PdfEnv {
	    office: pdf.Engines;
	    browser: string;
	
	    static createFrom(source: any = {}) {
	        return new PdfEnv(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.office = this.convertValues(source["office"], pdf.Engines);
	        this.browser = source["browser"];
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
	export class PdfJob {
	    id: string;
	    path: string;
	    password: string;
	    outDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new PdfJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.password = source["password"];
	        this.outDir = source["outDir"];
	    }
	}
	export class PdfaCheckOptions {
	    flavour: string;
	
	    static createFrom(source: any = {}) {
	        return new PdfaCheckOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flavour = source["flavour"];
	    }
	}
	export class PdfOptions {
	    compress: pdf.CompressOptions;
	    rotate: number;
	    protect: pdf.ProtectOptions;
	    watermark: pdf.WatermarkOptions;
	    numbers: pdf.NumberOptions;
	    export: pdf.ImageExportOptions;
	    ocr: pdf.OCROptions;
	    crop: pdf.CropOptions;
	    split: pdf.SplitOptions;
	    pages: string;
	    separate: boolean;
	    html: pdf.HTMLOptions;
	    images: pdf.ImagesOptions;
	    digisign: pdf.DigitalSignOptions;
	    pdfaCheck: PdfaCheckOptions;
	    headerFooter: pdf.HeaderFooterOptions;
	    nup: pdf.NUpOptions;
	    semicolon: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PdfOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.compress = this.convertValues(source["compress"], pdf.CompressOptions);
	        this.rotate = source["rotate"];
	        this.protect = this.convertValues(source["protect"], pdf.ProtectOptions);
	        this.watermark = this.convertValues(source["watermark"], pdf.WatermarkOptions);
	        this.numbers = this.convertValues(source["numbers"], pdf.NumberOptions);
	        this.export = this.convertValues(source["export"], pdf.ImageExportOptions);
	        this.ocr = this.convertValues(source["ocr"], pdf.OCROptions);
	        this.crop = this.convertValues(source["crop"], pdf.CropOptions);
	        this.split = this.convertValues(source["split"], pdf.SplitOptions);
	        this.pages = source["pages"];
	        this.separate = source["separate"];
	        this.html = this.convertValues(source["html"], pdf.HTMLOptions);
	        this.images = this.convertValues(source["images"], pdf.ImagesOptions);
	        this.digisign = this.convertValues(source["digisign"], pdf.DigitalSignOptions);
	        this.pdfaCheck = this.convertValues(source["pdfaCheck"], PdfaCheckOptions);
	        this.headerFooter = this.convertValues(source["headerFooter"], pdf.HeaderFooterOptions);
	        this.nup = this.convertValues(source["nup"], pdf.NUpOptions);
	        this.semicolon = source["semicolon"];
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
	
	export class PendingSummary {
	    batches: number;
	    items: number;
	    titles: string[];
	
	    static createFrom(source: any = {}) {
	        return new PendingSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.batches = source["batches"];
	        this.items = source["items"];
	        this.titles = source["titles"];
	    }
	}
	export class Recipe {
	    kind: string;
	    id: string;
	    name: string;
	    builtin: boolean;
	    job: number[];
	
	    static createFrom(source: any = {}) {
	        return new Recipe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.builtin = source["builtin"];
	        this.job = source["job"];
	    }
	}
	export class SheetOptions {
	    cols: number;
	    rows: number;
	    width: number;
	    format: string;
	    times: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SheetOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	        this.width = source["width"];
	        this.format = source["format"];
	        this.times = source["times"];
	    }
	}
	export class SubscriptionInfo {
	    id: string;
	    url: string;
	    title: string;
	    source: string;
	    type: string;
	    thumbnail: string;
	    mode: string;
	    tabs: string[];
	    everyHours: number;
	    enabled: boolean;
	    lastCheck: number;
	    nextCheck: number;
	    lastNew: number;
	    totalNew: number;
	    lastError: string;
	    checking: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.source = source["source"];
	        this.type = source["type"];
	        this.thumbnail = source["thumbnail"];
	        this.mode = source["mode"];
	        this.tabs = source["tabs"];
	        this.everyHours = source["everyHours"];
	        this.enabled = source["enabled"];
	        this.lastCheck = source["lastCheck"];
	        this.nextCheck = source["nextCheck"];
	        this.lastNew = source["lastNew"];
	        this.totalNew = source["totalNew"];
	        this.lastError = source["lastError"];
	        this.checking = source["checking"];
	    }
	}
	export class SubtitleJob {
	    model: string;
	    language: string;
	    translate: boolean;
	    formats: string[];
	    maxLen: number;
	    video: string;
	
	    static createFrom(source: any = {}) {
	        return new SubtitleJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.language = source["language"];
	        this.translate = source["translate"];
	        this.formats = source["formats"];
	        this.maxLen = source["maxLen"];
	        this.video = source["video"];
	    }
	}
	export class VideoJob {
	    mode: string;
	    video: mediaconv.VideoOptions;
	    audio: mediaconv.AudioOptions;
	    frameEvery: number;
	    frameFormat: string;
	    sheet: SheetOptions;
	
	    static createFrom(source: any = {}) {
	        return new VideoJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.video = this.convertValues(source["video"], mediaconv.VideoOptions);
	        this.audio = this.convertValues(source["audio"], mediaconv.AudioOptions);
	        this.frameEvery = source["frameEvery"];
	        this.frameFormat = source["frameFormat"];
	        this.sheet = this.convertValues(source["sheet"], SheetOptions);
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
	    vbr: boolean;
	    vbrLevel: string;
	    channels: string;
	    sampleRate: string;
	    keepMetadata: boolean;
	    trimStart: string;
	    trimEnd: string;
	    fadeIn: number;
	    fadeOut: number;
	    normalize: boolean;
	    loudness: number;
	    removeSilence: boolean;
	    speed: number;
	    pitch: number;
	    denoise: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.bitrate = source["bitrate"];
	        this.vbr = source["vbr"];
	        this.vbrLevel = source["vbrLevel"];
	        this.channels = source["channels"];
	        this.sampleRate = source["sampleRate"];
	        this.keepMetadata = source["keepMetadata"];
	        this.trimStart = source["trimStart"];
	        this.trimEnd = source["trimEnd"];
	        this.fadeIn = source["fadeIn"];
	        this.fadeOut = source["fadeOut"];
	        this.normalize = source["normalize"];
	        this.loudness = source["loudness"];
	        this.removeSilence = source["removeSilence"];
	        this.speed = source["speed"];
	        this.pitch = source["pitch"];
	        this.denoise = source["denoise"];
	    }
	}
	export class CropBox {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new CropBox(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class Music {
	    file: string;
	    mode: string;
	    volume: number;
	    original: number;
	    duck: boolean;
	    loop: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Music(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.mode = source["mode"];
	        this.volume = source["volume"];
	        this.original = source["original"];
	        this.duck = source["duck"];
	        this.loop = source["loop"];
	    }
	}
	export class SlideOptions {
	    format: string;
	    seconds: number;
	    size: number;
	    ratio: string;
	    fit: string;
	    background: string;
	    fade: number;
	    music: string;
	
	    static createFrom(source: any = {}) {
	        return new SlideOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.seconds = source["seconds"];
	        this.size = source["size"];
	        this.ratio = source["ratio"];
	        this.fit = source["fit"];
	        this.background = source["background"];
	        this.fade = source["fade"];
	        this.music = source["music"];
	    }
	}
	export class Tags {
	    title: string;
	    artist: string;
	    album: string;
	    albumArtist: string;
	    year: string;
	    genre: string;
	    track: string;
	    cover: string;
	    removeCover: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tags(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.albumArtist = source["albumArtist"];
	        this.year = source["year"];
	        this.genre = source["genre"];
	        this.track = source["track"];
	        this.cover = source["cover"];
	        this.removeCover = source["removeCover"];
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
	    targetMB: number;
	    bitrateK: number;
	    hw: string;
	    trimStart: string;
	    trimEnd: string;
	    fps: string;
	    fpsCustom: number;
	    audioMode: string;
	    audioBitrate: number;
	    rotate: number;
	    flipH: boolean;
	    flipV: boolean;
	    subtitles: string;
	    speed: number;
	    reverse: boolean;
	    crop: CropBox;
	    frame: string;
	    frameFit: string;
	    stabilize: boolean;
	    denoise: string;
	    music: Music;
	    watermark: imageconv.Watermark;
	
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
	        this.targetMB = source["targetMB"];
	        this.bitrateK = source["bitrateK"];
	        this.hw = source["hw"];
	        this.trimStart = source["trimStart"];
	        this.trimEnd = source["trimEnd"];
	        this.fps = source["fps"];
	        this.fpsCustom = source["fpsCustom"];
	        this.audioMode = source["audioMode"];
	        this.audioBitrate = source["audioBitrate"];
	        this.rotate = source["rotate"];
	        this.flipH = source["flipH"];
	        this.flipV = source["flipV"];
	        this.subtitles = source["subtitles"];
	        this.speed = source["speed"];
	        this.reverse = source["reverse"];
	        this.crop = this.convertValues(source["crop"], CropBox);
	        this.frame = source["frame"];
	        this.frameFit = source["frameFit"];
	        this.stabilize = source["stabilize"];
	        this.denoise = source["denoise"];
	        this.music = this.convertValues(source["music"], Music);
	        this.watermark = this.convertValues(source["watermark"], imageconv.Watermark);
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

export namespace pdf {
	
	export class Bookmark {
	    title: string;
	    page: number;
	    kids: Bookmark[];
	
	    static createFrom(source: any = {}) {
	        return new Bookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.page = source["page"];
	        this.kids = this.convertValues(source["kids"], Bookmark);
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
	export class CertInfo {
	    name: string;
	    email: string;
	    issuer: string;
	    selfSigned: boolean;
	    notBefore: string;
	    notAfter: string;
	
	    static createFrom(source: any = {}) {
	        return new CertInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.issuer = source["issuer"];
	        this.selfSigned = source["selfSigned"];
	        this.notBefore = source["notBefore"];
	        this.notAfter = source["notAfter"];
	    }
	}
	export class Rect {
	    page: number;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class Change {
	    kind: string;
	    text: string;
	    page: number;
	    boxes: Rect[];
	
	    static createFrom(source: any = {}) {
	        return new Change(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.text = source["text"];
	        this.page = source["page"];
	        this.boxes = this.convertValues(source["boxes"], Rect);
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
	export class CompareResult {
	    pagesA: number;
	    pagesB: number;
	    changes: Change[];
	    added: number;
	    removed: number;
	    same: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CompareResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pagesA = source["pagesA"];
	        this.pagesB = source["pagesB"];
	        this.changes = this.convertValues(source["changes"], Change);
	        this.added = source["added"];
	        this.removed = source["removed"];
	        this.same = source["same"];
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
	export class CompressOptions {
	    level: string;
	    gray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CompressOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.gray = source["gray"];
	    }
	}
	export class CropOptions {
	    mode: string;
	    top: number;
	    right: number;
	    bottom: number;
	    left: number;
	    box: Rect;
	    pages: string;
	
	    static createFrom(source: any = {}) {
	        return new CropOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.top = source["top"];
	        this.right = source["right"];
	        this.bottom = source["bottom"];
	        this.left = source["left"];
	        this.box = this.convertValues(source["box"], Rect);
	        this.pages = source["pages"];
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
	export class DigitalSignOptions {
	    certFile: string;
	    certPassword: string;
	    name: string;
	    reason: string;
	    location: string;
	    contact: string;
	    visible: boolean;
	    position: string;
	    page: string;
	    tsa: string;
	
	    static createFrom(source: any = {}) {
	        return new DigitalSignOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.certFile = source["certFile"];
	        this.certPassword = source["certPassword"];
	        this.name = source["name"];
	        this.reason = source["reason"];
	        this.location = source["location"];
	        this.contact = source["contact"];
	        this.visible = source["visible"];
	        this.position = source["position"];
	        this.page = source["page"];
	        this.tsa = source["tsa"];
	    }
	}
	export class Meta {
	    title: string;
	    author: string;
	    subject: string;
	    keywords: string;
	    creator: string;
	
	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.author = source["author"];
	        this.subject = source["subject"];
	        this.keywords = source["keywords"];
	        this.creator = source["creator"];
	    }
	}
	export class DocDetails {
	    meta: Meta;
	    bookmarks: Bookmark[];
	    pages: number;
	
	    static createFrom(source: any = {}) {
	        return new DocDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.meta = this.convertValues(source["meta"], Meta);
	        this.bookmarks = this.convertValues(source["bookmarks"], Bookmark);
	        this.pages = source["pages"];
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
	export class PageSize {
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new PageSize(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class DocInfo {
	    path: string;
	    name: string;
	    pages: PageSize[];
	    encrypted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DocInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.pages = this.convertValues(source["pages"], PageSize);
	        this.encrypted = source["encrypted"];
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
	export class Engines {
	    word: boolean;
	    excel: boolean;
	    powerpoint: boolean;
	    libreoffice: string;
	
	    static createFrom(source: any = {}) {
	        return new Engines(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.word = source["word"];
	        this.excel = source["excel"];
	        this.powerpoint = source["powerpoint"];
	        this.libreoffice = source["libreoffice"];
	    }
	}
	export class FormField {
	    id: string;
	    name: string;
	    kind: string;
	    value: string;
	    values: string[];
	    checked: boolean;
	    options: string[];
	    multiline: boolean;
	    multi: boolean;
	    locked: boolean;
	    page: number;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new FormField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.value = source["value"];
	        this.values = source["values"];
	        this.checked = source["checked"];
	        this.options = source["options"];
	        this.multiline = source["multiline"];
	        this.multi = source["multi"];
	        this.locked = source["locked"];
	        this.page = source["page"];
	        this.format = source["format"];
	    }
	}
	export class HTMLOptions {
	    pageSize: string;
	    orientation: string;
	    margin: string;
	    width: number;
	    onePage: boolean;
	    background: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HTMLOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pageSize = source["pageSize"];
	        this.orientation = source["orientation"];
	        this.margin = source["margin"];
	        this.width = source["width"];
	        this.onePage = source["onePage"];
	        this.background = source["background"];
	    }
	}
	export class HeaderFooterOptions {
	    topLeft: string;
	    topCenter: string;
	    topRight: string;
	    bottomLeft: string;
	    bottomCenter: string;
	    bottomRight: string;
	    size: number;
	    color: string;
	    bold: boolean;
	    margin: number;
	    line: boolean;
	    pages: string;
	    skipFirst: boolean;
	    mirror: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HeaderFooterOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topLeft = source["topLeft"];
	        this.topCenter = source["topCenter"];
	        this.topRight = source["topRight"];
	        this.bottomLeft = source["bottomLeft"];
	        this.bottomCenter = source["bottomCenter"];
	        this.bottomRight = source["bottomRight"];
	        this.size = source["size"];
	        this.color = source["color"];
	        this.bold = source["bold"];
	        this.margin = source["margin"];
	        this.line = source["line"];
	        this.pages = source["pages"];
	        this.skipFirst = source["skipFirst"];
	        this.mirror = source["mirror"];
	    }
	}
	export class ImageExportOptions {
	    mode: string;
	    format: string;
	    dpi: number;
	    quality: number;
	    pages: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.format = source["format"];
	        this.dpi = source["dpi"];
	        this.quality = source["quality"];
	        this.pages = source["pages"];
	    }
	}
	export class ImagesOptions {
	    pageSize: string;
	    orientation: string;
	    margin: string;
	    quality: number;
	    combine: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImagesOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pageSize = source["pageSize"];
	        this.orientation = source["orientation"];
	        this.margin = source["margin"];
	        this.quality = source["quality"];
	        this.combine = source["combine"];
	    }
	}
	export class Input {
	    path: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new Input(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.password = source["password"];
	    }
	}
	
	export class NUpOptions {
	    mode: string;
	    n: number;
	    paper: string;
	    border: boolean;
	    margin: number;
	
	    static createFrom(source: any = {}) {
	        return new NUpOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.n = source["n"];
	        this.paper = source["paper"];
	        this.border = source["border"];
	        this.margin = source["margin"];
	    }
	}
	export class NumberOptions {
	    position: string;
	    margin: number;
	    start: number;
	    pages: string;
	    format: string;
	    size: number;
	    color: string;
	    bold: boolean;
	    mirror: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NumberOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = source["position"];
	        this.margin = source["margin"];
	        this.start = source["start"];
	        this.pages = source["pages"];
	        this.format = source["format"];
	        this.size = source["size"];
	        this.color = source["color"];
	        this.bold = source["bold"];
	        this.mirror = source["mirror"];
	    }
	}
	export class OCRLanguage {
	    tag: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new OCRLanguage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag = source["tag"];
	        this.name = source["name"];
	    }
	}
	export class OCROptions {
	    lang: string;
	    pages: string;
	    skipText: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OCROptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lang = source["lang"];
	        this.pages = source["pages"];
	        this.skipText = source["skipText"];
	    }
	}
	export class PageRef {
	    src: number;
	    page: number;
	    rotate: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new PageRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.src = source["src"];
	        this.page = source["page"];
	        this.rotate = source["rotate"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	
	export class ProtectOptions {
	    password: string;
	    ownerPassword: string;
	    allowPrint: boolean;
	    allowCopy: boolean;
	    allowEdit: boolean;
	    aes128: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProtectOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.password = source["password"];
	        this.ownerPassword = source["ownerPassword"];
	        this.allowPrint = source["allowPrint"];
	        this.allowCopy = source["allowCopy"];
	        this.allowEdit = source["allowEdit"];
	        this.aes128 = source["aes128"];
	    }
	}
	
	export class RedactOptions {
	    boxes: Rect[];
	    dpi: number;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new RedactOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.boxes = this.convertValues(source["boxes"], Rect);
	        this.dpi = source["dpi"];
	        this.color = source["color"];
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
	export class SplitOptions {
	    mode: string;
	    ranges: string;
	    every: number;
	
	    static createFrom(source: any = {}) {
	        return new SplitOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.ranges = source["ranges"];
	        this.every = source["every"];
	    }
	}
	export class WatermarkOptions {
	    type: string;
	    text: string;
	    size: number;
	    bold: boolean;
	    color: string;
	    image: string;
	    scale: number;
	    opacity: number;
	    angle: number;
	    position: string;
	    pages: string;
	    under: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WatermarkOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.text = source["text"];
	        this.size = source["size"];
	        this.bold = source["bold"];
	        this.color = source["color"];
	        this.image = source["image"];
	        this.scale = source["scale"];
	        this.opacity = source["opacity"];
	        this.angle = source["angle"];
	        this.position = source["position"];
	        this.pages = source["pages"];
	        this.under = source["under"];
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
	    input: string;
	    inSize: number;
	    output: string;
	    outSize: number;
	    started: number;
	    finished: number;
	    seq: number;
	
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
	        this.input = source["input"];
	        this.inSize = source["inSize"];
	        this.output = source["output"];
	        this.outSize = source["outSize"];
	        this.started = source["started"];
	        this.finished = source["finished"];
	        this.seq = source["seq"];
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
	    optional: boolean;
	
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
	        this.optional = source["optional"];
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

