import { L } from './i18n.svelte'

/** How a tool's workspace looks. */
export type ToolPattern = 'batch' | 'combine' | 'pages' | 'editor' | 'compare' | 'web'

/** File filter sent to the backend (see app_pdf.go). */
export type ToolInput = 'pdf' | 'pdfimage' | 'word' | 'excel' | 'ppt' | 'html'

export interface PdfTool {
  id: string
  cat: string
  icon: string
  color: string
  pattern: ToolPattern
  input: ToolInput
  formats: string[]
  name: () => string
  desc: () => string
}

export const categories: { id: string; name: () => string }[] = [
  { id: 'organize', name: () => L('Atur halaman', 'Organize pages') },
  { id: 'optimize', name: () => L('Optimasi', 'Optimize') },
  { id: 'to', name: () => L('Ubah ke PDF', 'Convert to PDF') },
  { id: 'from', name: () => L('Ubah dari PDF', 'Convert from PDF') },
  { id: 'edit', name: () => L('Edit & tandai', 'Edit & mark up') },
  { id: 'security', name: () => L('Keamanan', 'Security') },
]

const PDF = ['PDF']
const IMG = ['JPG', 'PNG', 'WEBP', 'HEIC', 'AVIF', 'BMP', 'TIFF', 'GIF']

// Tile colours per category (background, foreground), matching the dark theme.
const tone: Record<string, string> = {
  organize: '#2B3A55|#B9C8E4',
  optimize: '#2F4A3E|#C0E2D0',
  to: '#4A3B2B|#E8D2B8',
  from: '#3B2F4A|#D2C2E6',
  edit: '#2B4448|#BCE0E4',
  security: '#4A2B32|#E8B8C2',
}

function t(id: string, cat: string, icon: string, pattern: ToolPattern, input: ToolInput, formats: string[], name: () => string, desc: () => string): PdfTool {
  return { id, cat, icon, color: tone[cat], pattern, input, formats, name, desc }
}

export const pdfTools: PdfTool[] = [
  t('merge', 'organize', 'layers', 'combine', 'pdf', PDF, () => L('Gabungkan PDF', 'Merge PDF'), () => L('Satukan beberapa PDF jadi satu, urutannya bisa diatur.', 'Join several PDFs into one, in the order you want.')),
  t('split', 'organize', 'scissors', 'pages', 'pdf', PDF, () => L('Pisahkan PDF', 'Split PDF'), () => L('Pecah PDF per rentang halaman, tiap N halaman, atau per halaman.', 'Split by page ranges, every N pages, or page by page.')),
  t('remove', 'organize', 'fileMinus', 'pages', 'pdf', PDF, () => L('Hapus halaman', 'Remove pages'), () => L('Buang halaman yang tidak diperlukan.', 'Delete the pages you don\'t need.')),
  t('extract', 'organize', 'fileOutput', 'pages', 'pdf', PDF, () => L('Ekstrak halaman', 'Extract pages'), () => L('Ambil halaman tertentu menjadi PDF baru.', 'Take chosen pages into a new PDF.')),
  t('organize', 'organize', 'grid', 'pages', 'pdf', PDF, () => L('Susun halaman', 'Organize PDF'), () => L('Geser, putar, hapus, gandakan halaman, atau tambah halaman kosong.', 'Drag, rotate, delete or duplicate pages and add blank ones.')),
  t('scan', 'organize', 'scan', 'combine', 'pdfimage', IMG, () => L('Scan ke PDF', 'Scan to PDF'), () => L('Ambil dari scanner atau kamera, lalu jadikan PDF.', 'Capture from a scanner or camera and save as PDF.')),

  t('compress', 'optimize', 'shrink', 'batch', 'pdf', PDF, () => L('Kompres PDF', 'Compress PDF'), () => L('Perkecil ukuran file dengan tiga tingkat kompresi.', 'Make files smaller with three compression levels.')),
  t('repair', 'optimize', 'wrench', 'batch', 'pdf', PDF, () => L('Perbaiki PDF', 'Repair PDF'), () => L('Selamatkan PDF rusak yang tidak bisa dibuka.', 'Recover damaged PDFs that won\'t open.')),
  t('ocr', 'optimize', 'textScan', 'batch', 'pdf', PDF, () => L('OCR PDF', 'OCR PDF'), () => L('Jadikan PDF hasil scan bisa dicari dan disalin teksnya.', 'Make scanned PDFs searchable and selectable.')),

  t('img2pdf', 'to', 'image', 'combine', 'pdfimage', IMG, () => L('Gambar ke PDF', 'Images to PDF'), () => L('JPG, PNG, HEIC, dll. ke PDF dengan ukuran kertas dan margin.', 'JPG, PNG, HEIC and more to PDF with paper size and margins.')),
  t('word2pdf', 'to', 'fileText', 'batch', 'word', ['DOC', 'DOCX', 'ODT', 'RTF'], () => L('Word ke PDF', 'Word to PDF'), () => L('Dokumen Word jadi PDF dengan tata letak tetap.', 'Word documents to PDF, layout preserved.')),
  t('ppt2pdf', 'to', 'presentation', 'batch', 'ppt', ['PPT', 'PPTX', 'ODP'], () => L('PowerPoint ke PDF', 'PowerPoint to PDF'), () => L('Slide presentasi jadi PDF.', 'Presentation slides to PDF.')),
  t('excel2pdf', 'to', 'table', 'batch', 'excel', ['XLS', 'XLSX', 'ODS', 'CSV'], () => L('Excel ke PDF', 'Excel to PDF'), () => L('Lembar kerja Excel jadi PDF.', 'Excel spreadsheets to PDF.')),
  t('html', 'to', 'globe', 'web', 'html', ['URL', 'HTML'], () => L('HTML ke PDF', 'HTML to PDF'), () => L('Simpan halaman web atau file HTML sebagai PDF.', 'Save web pages or HTML files as PDF.')),

  t('pdf2img', 'from', 'image', 'batch', 'pdf', PDF, () => L('PDF ke gambar', 'PDF to images'), () => L('Ubah tiap halaman jadi JPG/PNG, atau ambil gambar di dalamnya.', 'Turn pages into JPG/PNG, or pull out the pictures inside.')),
  t('pdf2word', 'from', 'fileText', 'batch', 'pdf', PDF, () => L('PDF ke Word', 'PDF to Word'), () => L('Ubah PDF jadi dokumen Word yang bisa diedit.', 'Turn PDFs into editable Word documents.')),
  t('pdf2ppt', 'from', 'presentation', 'batch', 'pdf', PDF, () => L('PDF ke PowerPoint', 'PDF to PowerPoint'), () => L('Setiap halaman jadi satu slide.', 'Every page becomes a slide.')),
  t('pdf2excel', 'from', 'table', 'batch', 'pdf', PDF, () => L('PDF ke Excel', 'PDF to Excel'), () => L('Tabel di PDF jadi lembar kerja Excel.', 'Tables in PDFs become Excel sheets.')),
  t('pdfa', 'from', 'archive', 'batch', 'pdf', PDF, () => L('PDF ke PDF/A', 'PDF to PDF/A'), () => L('Format arsip jangka panjang (PDF/A-2b).', 'Long-term archive format (PDF/A-2b).')),

  t('rotate', 'edit', 'rotateCw', 'batch', 'pdf', PDF, () => L('Putar PDF', 'Rotate PDF'), () => L('Putar semua atau sebagian halaman 90°, 180°, 270°.', 'Rotate all or some pages by 90°, 180° or 270°.')),
  t('numbers', 'edit', 'hash', 'batch', 'pdf', PDF, () => L('Nomor halaman', 'Page numbers'), () => L('Tambah nomor halaman dengan posisi dan format pilihan.', 'Add page numbers in the position and format you want.')),
  t('watermark', 'edit', 'stamp', 'batch', 'pdf', PDF, () => L('Tambah watermark', 'Add watermark'), () => L('Cap teks atau logo, transparan dan bisa diputar.', 'Stamp text or a logo, transparent and rotatable.')),
  t('edit', 'edit', 'pen', 'editor', 'pdf', PDF, () => L('Edit PDF', 'Edit PDF'), () => L('Tambah teks, kotak, lingkaran, garis, coretan, atau gambar.', 'Add text, boxes, circles, lines, drawings or pictures.')),
  t('crop', 'edit', 'crop', 'editor', 'pdf', PDF, () => L('Potong PDF', 'Crop PDF'), () => L('Pangkas margin atau pilih area halaman yang dipakai.', 'Trim margins or pick the page area to keep.')),

  t('unlock', 'security', 'unlock', 'batch', 'pdf', PDF, () => L('Buka kunci PDF', 'Unlock PDF'), () => L('Hapus password dan batasan cetak/salin.', 'Remove the password and print/copy restrictions.')),
  t('protect', 'security', 'lock', 'batch', 'pdf', PDF, () => L('Kunci PDF', 'Protect PDF'), () => L('Beri password dengan enkripsi AES 256-bit.', 'Add a password with AES 256-bit encryption.')),
  t('sign', 'security', 'signature', 'editor', 'pdf', PDF, () => L('Tanda tangan PDF', 'Sign PDF'), () => L('Bubuhkan tanda tangan gambar, tulisan tangan, nama, dan tanggal.', 'Place a drawn, typed or image signature and the date.')),
  t('redact', 'security', 'eyeOff', 'editor', 'pdf', PDF, () => L('Sensor PDF', 'Redact PDF'), () => L('Hitamkan informasi sensitif secara permanen.', 'Black out sensitive information for good.')),
  t('compare', 'security', 'columns', 'compare', 'pdf', PDF, () => L('Bandingkan PDF', 'Compare PDF'), () => L('Lihat teks yang ditambah dan dihapus antara dua versi.', 'See the text added and removed between two versions.')),
]

export function toolById(id: string | null): PdfTool | undefined {
  return pdfTools.find((x) => x.id === id)
}
