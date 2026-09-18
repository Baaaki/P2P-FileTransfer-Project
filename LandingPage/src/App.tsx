import { Architecture } from './components/Architecture'
import { Downloads } from './components/Downloads'
import { Faq } from './components/Faq'
import { Features } from './components/Features'
import { Footer } from './components/Footer'
import { Hero } from './components/Hero'
import { Nav } from './components/Nav'
import { ScrollStory } from './components/ScrollStory'

export default function App() {
  return (
    <>
      <Nav />
      <main>
        <Hero />
        <ScrollStory />
        <Features />
        <Architecture />
        <Downloads />
        <Faq />
      </main>
      <Footer />
    </>
  )
}
