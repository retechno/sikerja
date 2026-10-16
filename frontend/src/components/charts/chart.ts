import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
} from 'chart.js'

Chart.register(ArcElement, BarElement, CategoryScale, Filler, Legend, LineElement, LinearScale, PointElement, Tooltip)
Chart.defaults.font.family = '"Plus Jakarta Sans", system-ui, sans-serif'
Chart.defaults.color = '#5B6662'

export const COLORS = { primary: '#0E5A52', accent: '#E8A33D', soft: '#CFE3DF', line: '#E3E6E0' }
