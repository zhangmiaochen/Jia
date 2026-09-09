import { createRouter, createWebHistory } from 'vue-router'
import { familyRoutes } from './family'
import { recordRoutes } from './records'
import { memoryRoutes } from './memory'

export const router = createRouter({
  history: createWebHistory(),
  routes: [...familyRoutes, ...recordRoutes, ...memoryRoutes, { path: '/', redirect: '/home' }, { path: '/:pathMatch(.*)*', redirect: '/home' }],
})
